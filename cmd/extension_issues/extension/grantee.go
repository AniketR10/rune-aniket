package extension

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"os/user"
	"sync/atomic"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/logging"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/ernestrc/blue/issue"
	log "github.com/sirupsen/logrus"
	browserapi "unstable.build/go-tui/api/browser"
	browserextension "unstable.build/go-tui/api/browser/extension"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	schemeextension "unstable.build/go-tui/api/scheme/extension"
	storageextension "unstable.build/go-tui/api/storage/extension"
	textapi "unstable.build/go-tui/api/text"
	textextension "unstable.build/go-tui/api/text/extension"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/storage/cache"
	"unstable.build/go-tui/text"
	workspacedoc "unstable.build/go-tui/workspace/document"
)

const (
	defaultMaxSubjectLen   = 50
	defaultCreateIssueCmd  = "issueCreate"
	issueRefreshCmd        = "issueRefresh"
	initialEvictAllTimeout = 1 * time.Minute
)

var (
	requiredPermissions = []extension.Permission{
		extension.PermissionBrowserWindowManager,
		extension.PermissionBrowserNotifications,
		extension.PermissionBrowserResourceOpener,
		extension.PermissionSchemeManager,
		extension.PermissionStorage,
		extension.PermissionEditor,
	}
	defaultCommands = map[string]commandAll{
		defaultCreateIssueCmd: {
			man: textapi.CommandManual{
				Summary: "Create an issue report using the default issues template. It opens " +
					"up a new file tab with a yaml that must be filled in and saved to " +
					"create issue. Closing the file before saving it cancels the creation " +
					"of a new issue.",
			},
			handler: (*issuesGrantee).openEmptyIssueTemplate,
		},
		issueRefreshCmd: {
			man: textapi.CommandManual{
				Summary: "Refreshes the local issues cache. This is useful when user " +
					"knows that out-of-band changes have been made to the issue tracker.",
			},
			handler: (*issuesGrantee).issueRefresh,
		},
	}
	editorEvents = []textapi.EventType{textapi.EventTypeFlush, textapi.EventTypeClose}
)

// Grantee returns this extension's grantee and the permissions required to run it.
// It uses the local bluectl configuration to load the issue tracker's credentials.
func GranteeWithService(
	versionTag, scheme string,
	svcFn func(config.Config) (document.Service, error),
) (extension.Grantee, []extension.Permission) {
	m := yaml.Marshaler()
	defaultAuthor := getDefaultAuthor()
	defTemplate := issue.Report{Author: defaultAuthor}
	data, err := m.Marshal(defTemplate)
	if err != nil {
		panic(fmt.Errorf("marshal default issue template: %v", err))
	}
	s := &issuesGrantee{
		cmds:        defaultCommands,
		marshaler:   m,
		defTemplate: data,
		versionTag:  versionTag,
		scheme:      scheme,
		svcFn:       svcFn,
	}
	s.pendingIssueURI.Store(workspaceapi.URI{})
	return s, requiredPermissions
}

type issuesGrantee struct {
	config       config.Config
	broker       proto.MuxBroker
	svc          *cache.Service[issue.ReportDocument]
	tracker      issue.Tracker
	trackerError error
	marshaler    encoding.Marshaler
	m            browserapi.Notifications
	o            browserapi.ResourceOpener
	wm           browserapi.WindowManager
	sm           schemeapi.SchemeManager
	s            document.Service
	versionTag   string
	svcFn        func(config.Config) (document.Service, error)
	scheme       string

	cmds          map[string]commandAll
	maxSubjectLen int
	defTemplate   []byte

	pendingIssueID  string       // accessed by Handle only, no need to synchronize
	pendingIssueURI atomic.Value // workspaceapi.URI, accessed by Handle and HandleCommand
}

func (e *issuesGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.config = pconfig
	e.broker = broker
	var err error

	disableDefaultCommands, err := pconfig.GetBool("disable_default_commands")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warnf("could not read property "+
				"'disable_default_commands': %v", err)
		}
	}

	e.maxSubjectLen, err = pconfig.GetInt("max_list_files_subject_len")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warnf("could not read property "+
				"'max_list_files_subject_len': %v", err)
		}
		e.maxSubjectLen = defaultMaxSubjectLen
	}

	templatesMap, err := pconfig.GetMap("templates")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warnf("could not read property 'templates': %v", err)
		}
		return
	}
	cmdToTemplates := make(map[string][]byte, len(templatesMap))
	for cmd, templateIfc := range templatesMap {
		template, ok := templateIfc.(map[string]interface{})
		if !ok {
			log.Warnf("'templates' keys must be a string and values must be a map "+
				"representing the issue template: key %q found %v", cmd, templateIfc)
			continue
		}
		data, err := e.marshaler.Marshal(template)
		if err != nil {
			log.Warnf("'templates' keys must be a string and values must be a map "+
				"representing the issue template: could not marshal template for %q: %v", cmd, err)
			continue
		}

		// sort order of marshalled keys in template to
		// what we encounter via bluectl issue create
		var temp issue.Report
		err = e.marshaler.Unmarshal(data, &temp)
		if err != nil {
			log.Error(err)
			continue
		}
		// add default version via compile-time variable
		if temp.Version == "" {
			temp.Version = e.versionTag
		}
		data, err = e.marshaler.Marshal(temp)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugf("marshaled template %q into %s", cmd, string(data))
		cmdToTemplates[cmd] = data
	}

	if disableDefaultCommands {
		e.cmds = make(map[string]commandAll)
	}

	for cmd, template := range cmdToTemplates {
		e.cmds[cmd] = commandAll{
			man: textapi.CommandManual{
				Name: cmd,
				Summary: fmt.Sprintf("Create an issue report using a custom %q issue template. "+
					"See %s for more details.", cmd, defaultCreateIssueCmd),
			},
			handler: e.openCustomIssueTemplate(template, cmd),
		}
	}

	log.Debugf("extension connected and loaded config without any critical issues")
}

func (e *issuesGrantee) initScheme(m schemeapi.SchemeManager) error {
	marshaler := yaml.Marshaler()
	rootURI, err := workspaceapi.ParseURI(fmt.Sprintf("%s:///", e.scheme))
	if err != nil {
		panic(err)
	}

	schemeFn := workspacedoc.WorkspaceScheme[issue.ReportDocument](rootURI, e.svc,
		marshaler, fmt.Errorf("missing %q sub-field in Metadata field",
			issue.ReportMetadataIDField))
	schemeFn = issueMapperScheme(schemeFn, marshaler, e.maxSubjectLen)
	err = m.RegisterScheme(e.scheme, schemeFn)
	return err
}

func (e *issuesGrantee) Handle(ctx context.Context, ev textapi.Event) bool {
	pendingIssueURI := e.pendingIssueURI.Load().(workspaceapi.URI)
	if !ev.URI.Equal(pendingIssueURI) {
		log.Tracef("ignoring event for file with URI %q: not an issue URI", ev.URI)
		return false
	}

	log.Debugf("handling event %v for issue with URI %q", ev.Type, ev.URI)

	switch ev.Type {
	case textapi.EventTypeFlush:
		e.createOrUpdateIssue(ctx, ev)
	case textapi.EventTypeClose:
		e.freeIssue(ctx, ev, pendingIssueURI)
	}
	return false
}

func (e *issuesGrantee) PermissionGranted(grants []extension.Grant) {
	log.Debugf("permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.Permission(extension.PermissionBrowserWindowManager):
			wm, err := browserextension.WindowManager(g, e.broker)
			if err != nil {
				log.Warnf("Could not acquire browser window manager: %v. "+
					"Will not be able to create reports.", err)
				continue
			}
			e.wm = wm
		case extension.Permission(extension.PermissionBrowserResourceOpener):
			o, err := browserextension.ResourceOpener(g, e.broker)
			if err != nil {
				log.Warnf("Could not acquire browser resource opener: %v. "+
					"Will not be able to create reports.", err)
				continue
			}
			e.o = o
		case extension.Permission(extension.PermissionBrowserNotifications):
			m, err := browserextension.Notifications(g, e.broker)
			if err != nil {
				log.Warnf("Could not acquire browser messenger: %v. "+
					"Will not be able to report errors to user.", err)
				continue
			}
			e.m = m
		case extension.Permission(extension.PermissionEditor):
			ed, err := textextension.Editor(g, e.broker)
			if err != nil {
				log.Warnf("Could not acquire editor to subscribe command: %v."+
					" Will not be able to create reports", err)
				continue
			}
			err = ed.SubscribeEvents(editorEvents, e)
			if err != nil {
				log.Warnf("Could not subscribe to editor events: %v. "+
					"Will not be able to create reports", err)
				continue
			}
			for cmd, man := range e.cmds {
				cmd := cmd
				man := man
				man.man.Name = cmd
				err = ed.SubscribeCommand(man.man, text.FuncCommandHandler(
					func(ctx context.Context, cmd textapi.Command) (bool, error) {
						return man.handler(e, ctx, cmd)
					}))
				if err != nil {
					log.Warnf("Could not subscribe command %q: %v", cmd, err)
					continue
				}
				log.Debugf("Subscribed to create issue command %q", cmd)
			}
		case extension.PermissionSchemeManager:
			m, err := schemeextension.SchemeManager(g, e.broker)
			if err != nil {
				log.Warnf("Could not acquire scheme manager: %v. "+
					"Will not be able to create or see reports", err)
				continue
			}
			e.sm = m
		case extension.PermissionStorage:
			s, err := storageextension.Storage(g, e.broker)
			if err != nil {
				log.Warnf("Could not acquire storage: %v. "+
					"Will not be able to create or see reports", err)
				continue
			}
			e.s = s
		}
	}

	if e.sm == nil || e.s == nil {
		log.Errorf("missing critical resources, cannot continue.")
		return
	}

	svc, err := e.svcFn(e.config)
	if err != nil {
		log.Warn(err)
		e.trackerError = err
		return
	}
	svc = logging.WithLogging(svc, "IssuesStorage")

	// avoid too many reads to service (i.e. firestore), which is pretty slow.
	// As long as there aren't many oob (outside of six) requests this should be
	// able to cache expensive list + get all operations pretty well
	// because it's using the host's global storage as the cache
	e.svc = cache.New[issue.ReportDocument](svc, e.s)
	e.tracker = issue.NewDocumentTracker(e.svc)

	err = e.initScheme(e.sm)
	if err != nil {
		log.Warnf("Could not initialize scheme: %v. "+
			"Will not be able to list reports", err)
	}

	// this is just massaging the cache so evict async
	// to shorten time to initialize extension
	go func() {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, initialEvictAllTimeout)
		defer cancel()

		err = e.svc.EvictAll(ctx)
		if err != nil {
			log.Warnf("Could not initialize issue cache: %v", err)
			return
		}
		log.Debugf("initialized extension_issues successfully")
	}()
}

func (e *issuesGrantee) PermissionDenied(perms []extension.Permission) {
	log.Warningf("missing critical permissions: denied: %v; required: %v", perms, requiredPermissions)
}

func (e *issuesGrantee) Health() error {
	return nil
}

func (e *issuesGrantee) Shutdown(reason string) error {
	log.Debugf("extension being shutdown: %s", reason)
	return nil
}

func (e *issuesGrantee) notify(level notifications.Level, msg string, args ...any) {
	if e.m == nil {
		return
	}
	err := e.m.Notify(level, msg, args...)
	if err != nil {
		err = fmt.Errorf("set message: %v", err)
		log.Error(err)
	}
}

func (e *issuesGrantee) issueRefresh(ctx context.Context, cmd textapi.Command) (bool, error) {
	if e.trackerError != nil {
		return false, fmt.Errorf("initialize issue tracker: %v", e.trackerError)
	}
	if e.svc == nil {
		return false, errors.New("cannot refresh issues if permissions were not granted")
	}
	return false, e.svc.EvictAll(ctx)
}

func (e *issuesGrantee) openEmptyIssueTemplate(ctx context.Context, cmd textapi.Command) (bool, error) {
	return e.openIssueTemplate(ctx, cmd.Window, e.defTemplate, "issue-")
}

func (e *issuesGrantee) openCustomIssueTemplate(
	template []byte, templateName string,
) func(*issuesGrantee, context.Context, textapi.Command) (bool, error) {
	return func(e *issuesGrantee, ctx context.Context, cmd textapi.Command) (bool, error) {
		return e.openIssueTemplate(ctx, cmd.Window, template, templateName)
	}
}

func (e *issuesGrantee) freeIssue(ctx context.Context, ev textapi.Event, uri workspaceapi.URI) bool {
	if e.pendingIssueID == "" {
		e.notify(notifications.LevelInfo, "canceled creation of new issue")
	}
	e.pendingIssueURI.CompareAndSwap(uri, workspaceapi.URI{})
	_ = os.Remove(uri.Path())
	e.pendingIssueID = ""
	return false
}

func (e *issuesGrantee) createReport(ctx context.Context, temp issue.Report) string {
	id, err := e.tracker.CreateReport(ctx, temp)
	if err != nil {
		err = fmt.Errorf("create report: %v", err)
		e.notify(notifications.LevelError, err.Error())
		log.Error(err)
		return ""
	}
	msg := fmt.Sprintf("created issue report %s", id)
	e.notify(notifications.LevelSuccess, msg)
	log.Info(msg)
	return id
}

func (e *issuesGrantee) updateReport(ctx context.Context, id string, temp issue.Report) {
	err := e.tracker.UpdateReport(ctx, id, temp)
	if err != nil {
		err = fmt.Errorf("update report: %v", err)
		e.notify(notifications.LevelError, err.Error())
		log.Error(err)
		return
	}

	msg := fmt.Sprintf("updated issue report %s", id)
	e.notify(notifications.LevelSuccess, msg)
	log.Info(msg)
}

func (e *issuesGrantee) createOrUpdateIssue(ctx context.Context, ev textapi.Event) bool {
	if e.tracker == nil {
		msg := "Cannot create or update issue if permissions were denied " +
			"or there was an error initializing issue tracker client."
		log.Error(msg)
		return false
	}
	var temp issue.Report
	err := e.marshaler.Unmarshal([]byte(ev.Content), &temp)
	if err != nil {
		err = fmt.Errorf("unmarshal: %v", err)
		e.notify(notifications.LevelError, err.Error())
		log.Warn(err)
		return false
	}
	if e.pendingIssueID != "" {
		e.updateReport(ctx, e.pendingIssueID, temp)
		return false
	}

	id := e.createReport(ctx, temp)
	e.pendingIssueID = id
	return false
}

func (e *issuesGrantee) openIssueTemplate(
	ctx context.Context, win browserapi.Window, template []byte, templateName string,
) (bool, error) {
	if e.o == nil || e.wm == nil {
		return false, errors.New("browser permissions necessary to create an issue were not granted")
	}
	if e.trackerError != nil {
		return false, fmt.Errorf("initialize issue tracker: %v", e.trackerError)
	}
	oldURI := e.pendingIssueURI.Load().(workspaceapi.URI)
	if !oldURI.Equal(workspaceapi.URI{}) {
		return false, errors.New("there's already a pending issue open. " +
			"You should close it first before attempting to create a new one.")
	}

	f, err := ioutil.TempFile("", templateName)
	if err != nil {
		return false, fmt.Errorf("temp file: %v", err)
	}

	_, err = f.Write(template)
	if err != nil {
		return false, fmt.Errorf("write template to file: %v", err)
	}

	_ = f.Close()
	newURI, err := workspaceapi.CurrentUserHostURI(f.Name())
	if err != nil {
		return false, fmt.Errorf("URI: %v", err)
	}

	h, err := e.o.Open(newURI)
	if err != nil {
		_ = os.Remove(f.Name())
		return false, fmt.Errorf("open temp file: %v", err)
	}

	err = win.SetContent(h)
	if err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		_ = h.Close()
		_ = os.Remove(f.Name())
		return false, fmt.Errorf("set focus window content: %v", err)
	}

	e.pendingIssueURI.Store(newURI)

	return false, nil
}

func getDefaultAuthor() string {
	u, err := user.Current()
	if err != nil {
		u = &user.User{Username: "unknown"}
	}
	h, err := os.Hostname()
	if err != nil {
		h = "unknown-host"
	}
	return fmt.Sprintf("%s@%s", u.Username, h)
}

type commandAll struct {
	man     textapi.CommandManual
	handler func(*issuesGrantee, context.Context, textapi.Command) (bool, error)
}
