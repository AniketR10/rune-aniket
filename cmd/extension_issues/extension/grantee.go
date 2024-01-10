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
	doclogging "github.com/ernestrc/blue/document/logging"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/ernestrc/blue/issue"
	"github.com/ernestrc/blue/logging"
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
			handler: (*grantee).openEmptyIssueTemplate,
		},
		issueRefreshCmd: {
			man: textapi.CommandManual{
				Summary: "Refreshes the local issues cache. This is useful when user " +
					"knows that out-of-band changes have been made to the issue tracker.",
			},
			handler: (*grantee).issueRefresh,
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
	s := &grantee{
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

type grantee struct {
	config     config.Config
	broker     proto.MuxBroker
	svc        *cache.Service[issue.ReportDocument]
	tracker    issue.Tracker
	marshaler  encoding.Marshaler
	m          browserapi.Notifications
	o          browserapi.ResourceOpener
	wm         browserapi.WindowManager
	sm         schemeapi.SchemeManager
	s          document.Service
	versionTag string
	svcFn      func(config.Config) (document.Service, error)
	scheme     string

	cmds          map[string]commandAll
	maxSubjectLen int
	defTemplate   []byte

	pendingIssueID  string       // accessed by Handle only, no need to synchronize
	pendingIssueURI atomic.Value // workspaceapi.URI, accessed by Handle and HandleCommand
}

func (e *grantee) Connected(
	ctx context.Context, broker proto.MuxBroker, pconfig config.Config,
) error {
	e.config = pconfig
	e.broker = broker

	disableDefaultCommands, err := pconfig.GetBool("disable_default_commands")
	if err != nil {
		if err != config.ErrNotFound {
			return fmt.Errorf("could not read property "+
				"'disable_default_commands': %w", err)
		}
	}

	e.maxSubjectLen, err = pconfig.GetInt("max_list_files_subject_len")
	if err != nil {
		if err != config.ErrNotFound {
			return fmt.Errorf("could not read property "+
				"'max_list_files_subject_len': %w", err)
		}
		e.maxSubjectLen = defaultMaxSubjectLen
	}

	templatesMap, err := pconfig.GetMap("templates")
	if err != nil {
		if err != config.ErrNotFound {
			return fmt.Errorf("could not read property 'templates': %w", err)
		}
		return nil
	}

	cmdToTemplates := make(map[string][]byte, len(templatesMap))
	for cmd, templateIfc := range templatesMap {
		template, ok := templateIfc.(map[string]interface{})
		if !ok {
			return fmt.Errorf("'templates' keys must be a string and values must be a map "+
				"representing the issue template: key %q found %v", cmd, templateIfc)
		}
		data, err := e.marshaler.Marshal(template)
		if err != nil {
			return fmt.Errorf("'templates' keys must be a string and values must be a map "+
				"representing the issue template: could not marshal template for %q: %w", cmd, err)
		}

		// sort order of marshalled keys in template to
		// what we encounter via bluectl issue create
		var temp issue.Report
		err = e.marshaler.Unmarshal(data, &temp)
		if err != nil {
			return fmt.Errorf("unmarshal template '%s': %w", cmd, err)
		}
		// add default version via compile-time variable
		if temp.Version == "" {
			temp.Version = e.versionTag
		}
		data, err = e.marshaler.Marshal(temp)
		if err != nil {
			return fmt.Errorf("marshal template '%s' with version: %w", cmd, err)
		}

		e.log(log.TraceLevel, "marshaled template %q into %s", cmd, string(data))

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

	e.log(log.DebugLevel, "extension connected and loaded %d templates without any issues",
		len(cmdToTemplates))

	return nil
}

func (e *grantee) initScheme(m schemeapi.SchemeManager) error {
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

func (e *grantee) Handle(ctx context.Context, ev textapi.Event) bool {
	pendingIssueURI := e.pendingIssueURI.Load().(workspaceapi.URI)
	if !ev.URI.Equal(pendingIssueURI) {
		e.log(log.TraceLevel, "ignoring event for file with URI %q: not an issue URI", ev.URI)
		return false
	}

	e.log(log.TraceLevel, "handling event %v for issue with URI %q", ev.Type, ev.URI)

	switch ev.Type {
	case textapi.EventTypeFlush:
		e.createOrUpdateIssue(ctx, ev)
	case textapi.EventTypeClose:
		e.freeIssue(ctx, ev, pendingIssueURI)
	}
	return false
}

func (e *grantee) PermissionGranted(ctx context.Context, grants []extension.Grant) error {
	e.log(log.DebugLevel, "permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.Permission(extension.PermissionBrowserWindowManager):
			wm, err := browserextension.WindowManager(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire browser window manager: %w ", err)
			}
			e.wm = wm
		case extension.Permission(extension.PermissionBrowserResourceOpener):
			o, err := browserextension.ResourceOpener(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire browser resource opener: %w ", err)
			}
			e.o = o
		case extension.Permission(extension.PermissionBrowserNotifications):
			m, err := browserextension.Notifications(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire browser notifications: %w ", err)
			}
			e.m = m
		case extension.Permission(extension.PermissionEditor):
			ed, err := textextension.Editor(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire editor: %w ", err)
			}
			err = ed.SubscribeEvents(editorEvents, e)
			if err != nil {
				return fmt.Errorf("subscribe editor events: %w ", err)
			}
			for cmd, man := range e.cmds {
				cmd := cmd
				man := man
				man.man.Name = cmd
				err = ed.SubscribeCommand(man.man, textapi.NopCommandCompleter(
					func(ctx context.Context, cmd textapi.Command) (bool, error) {
						return man.handler(e, ctx, cmd)
					}))
				if err != nil {
					return fmt.Errorf("subscribe command %q: %w", cmd, err)
				}
				e.log(log.DebugLevel, "Subscribed to create issue command %q", cmd)
			}
		case extension.PermissionSchemeManager:
			m, err := schemeextension.SchemeManager(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire scheme manager: %w", err)
			}
			e.sm = m
		case extension.PermissionStorage:
			s, err := storageextension.Storage(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire storage: %w", err)
			}
			e.s = s
		}
	}

	svc, err := e.svcFn(e.config)
	if err != nil {
		return fmt.Errorf("create issues service: %w", err)
	}
	svc = doclogging.WithLogging(svc, "IssuesStorage")

	// avoid too many reads to service (i.e. firestore), which is pretty slow.
	// As long as there aren't many oob (outside of six) requests this should be
	// able to cache expensive list + get all operations pretty well
	// because it's using the host's global storage as the cache
	e.svc = cache.New[issue.ReportDocument](svc, e.s)
	e.tracker = issue.NewDocumentTracker(e.svc)

	err = e.initScheme(e.sm)
	if err != nil {
		return fmt.Errorf("initialize scheme: %w", err)
	}

	// this is just massaging the cache so evict async
	// to shorten time to initialize extension
	go func() {
		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, initialEvictAllTimeout)
		defer cancel()

		err = e.svc.EvictAll(ctx)
		if err != nil {
			e.log(log.WarnLevel, "could not initialize issue cache: %v", err)
			return
		}
		e.log(log.DebugLevel, "initialized successfully")
	}()

	return nil
}

func (e *grantee) PermissionDenied(ctx context.Context, perms []extension.Permission) error {
	return fmt.Errorf("missing critical permissions: denied: %v; required: %v",
		perms, requiredPermissions)
}

func (e *grantee) Health(context.Context) error {
	return nil
}

func (e *grantee) Shutdown(ctx context.Context, reason string) error {
	e.log(log.DebugLevel, "shutdown: %s", reason)
	return nil
}

func (e *grantee) notify(level notifications.Level, msg string, args ...any) {
	err := e.m.Notify(level, msg, args...)
	if err != nil {
		e.log(log.ErrorLevel, "notify: %v", err)
	}
}

func (e *grantee) issueRefresh(ctx context.Context, cmd textapi.Command) (bool, error) {
	return false, e.svc.EvictAll(ctx)
}

func (e *grantee) openEmptyIssueTemplate(ctx context.Context, cmd textapi.Command) (bool, error) {
	return e.openIssueTemplate(ctx, cmd.Window, e.defTemplate, "issue-")
}

func (e *grantee) openCustomIssueTemplate(
	template []byte, templateName string,
) func(*grantee, context.Context, textapi.Command) (bool, error) {
	return func(e *grantee, ctx context.Context, cmd textapi.Command) (bool, error) {
		return e.openIssueTemplate(ctx, cmd.Window, template, templateName)
	}
}

func (e *grantee) freeIssue(ctx context.Context, ev textapi.Event, uri workspaceapi.URI) bool {
	if e.pendingIssueID == "" {
		e.notify(notifications.LevelInfo, "canceled creation of new issue")
	}
	e.pendingIssueURI.CompareAndSwap(uri, workspaceapi.URI{})
	_ = os.Remove(uri.Path())
	e.pendingIssueID = ""
	return false
}

func (e *grantee) createReport(ctx context.Context, temp issue.Report) string {
	id, err := e.tracker.CreateReport(ctx, temp)
	if err != nil {
		err = fmt.Errorf("create report: %w", err)
		e.notify(notifications.LevelError, err.Error())
		e.log(log.ErrorLevel, err.Error())
		return ""
	}
	msg := fmt.Sprintf("created issue report %s", id)
	e.notify(notifications.LevelSuccess, msg)
	e.log(log.InfoLevel, msg)
	return id
}

func (e *grantee) updateReport(ctx context.Context, id string, temp issue.Report) {
	err := e.tracker.UpdateReport(ctx, id, temp)
	if err != nil {
		err = fmt.Errorf("update report: %v", err)
		e.notify(notifications.LevelError, err.Error())
		e.log(log.ErrorLevel, err.Error())
		return
	}

	msg := fmt.Sprintf("updated issue report %s", id)
	e.notify(notifications.LevelSuccess, msg)
	e.log(log.InfoLevel, msg)
}

func (e *grantee) createOrUpdateIssue(ctx context.Context, ev textapi.Event) bool {
	var temp issue.Report
	err := e.marshaler.Unmarshal([]byte(ev.Content), &temp)
	if err != nil {
		err = fmt.Errorf("unmarshal: %v", err)
		e.notify(notifications.LevelError, err.Error())
		e.log(log.WarnLevel, err.Error())
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

func (e *grantee) openIssueTemplate(
	ctx context.Context, win browserapi.Window, template []byte, templateName string,
) (bool, error) {
	if e.o == nil || e.wm == nil {
		return false, errors.New("browser permissions necessary to create an issue were not granted")
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

func (t *grantee) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "issuesextension.grantee",
	}).Logf(level, msg, args...)
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
	handler func(*grantee, context.Context, textapi.Command) (bool, error)
}
