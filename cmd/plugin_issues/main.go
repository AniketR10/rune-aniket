package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/user"
	"path"
	"sync"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/firestore"
	"github.com/ernestrc/blue/document/logging"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/ernestrc/blue/issue"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/storage/cache"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
	workspacedoc "unstable.build/go-tui/workspace/document"
)

const (
	issuesScheme           = "bluectl+issues"
	defaultMaxSubjectLen   = 50
	defaultCreateIssueCmd  = "issueCreate"
	initialEvictAllTimeout = 1 * time.Minute
)

var (
	// compile-time variable
	Tag = "development"

	requiredPermissions = []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionSchemeManager,
		plugin.PermissionStorage,
		plugin.PermissionEditor,
	}
	defaultCommands = map[string]func(*issuesGrantee,
		context.Context, text.Command) (bool, error){
		defaultCreateIssueCmd: (*issuesGrantee).openEmptyIssueTemplate,
		"issueRefresh":        (*issuesGrantee).issueRefresh,
	}
	editorEvents = []text.EventType{text.EventTypeFlush, text.EventTypeClose}
)

type issuesGrantee struct {
	mu        sync.Mutex
	broker    proto.MuxBroker
	svc       *cache.Service[issue.ReportDocument]
	tracker   issue.Tracker
	marshaler encoding.Marshaler
	m         browser.Messenger
	o         browser.ResourceOpener
	wm        browser.WindowManager
	sm        workspace.SchemeManager
	s         document.Service

	cmds              map[string]func(*issuesGrantee, context.Context, text.Command) (bool, error)
	bluectlConfigFile string
	maxSubjectLen     int
	defTemplate       []byte

	pendingIssueURI workspace.URI
	pendingIssueID  string
}

func (e *issuesGrantee) Connected(broker proto.MuxBroker, pconfig config.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.broker = broker
	var err error
	e.bluectlConfigFile, err = pconfig.GetString("bluectl_config")
	if err != nil {
		if err != config.ErrNotFound {
			log.Warnf("could not read property 'bluectl_config': %v", err)
		}
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("no 'bluectl_config' provided and failed "+
				"to get user home dir: %v", err)
		}
		e.bluectlConfigFile = path.Join(homeDir, ".bluectl", "config")
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
			temp.Version = Tag
		}
		data, err = e.marshaler.Marshal(temp)
		if err != nil {
			log.Error(err)
			continue
		}
		log.Debugf("marshaled template %q into %s", cmd, string(data))
		cmdToTemplates[cmd] = data
	}

	for cmd, template := range cmdToTemplates {
		e.cmds[cmd] = e.openCustomIssueTemplate(template, cmd)
	}

	log.Debugf("plugin connected and loaded config without any major issues")
}

func (e *issuesGrantee) initScheme(m workspace.SchemeManager) error {
	marshaler := yaml.Marshaler()
	rootURI, err := workspace.ParseURI("bluectl+issues:///")
	if err != nil {
		panic(err)
	}

	schemeFn := workspacedoc.WorkspaceScheme[issue.ReportDocument](rootURI, e.svc,
		marshaler, fmt.Errorf("missing %q sub-field in Metadata field",
			issue.ReportMetadataIDField))
	schemeFn = issueMapperScheme(schemeFn, marshaler, e.maxSubjectLen)
	err = m.RegisterScheme(issuesScheme, schemeFn)
	return err
}

func (e *issuesGrantee) Handle(ctx context.Context, ev text.Event) bool {
	if !ev.URI.Equal(e.pendingIssueURI) {
		log.Tracef("ignoring event for file with URI %q: not an issue URI", ev.URI)
		return false
	}

	log.Debugf("handling event %v for issue with URI %q", ev.Type, ev.URI)

	switch ev.Type {
	case text.EventTypeFlush:
		e.createOrUpdateIssue(ctx, ev)
	case text.EventTypeClose:
		e.freeIssue(ctx, ev)
	}
	return false
}

func (e *issuesGrantee) PermissionGranted(grants []plugin.Grant) {
	log.Infof("permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case plugin.PermissionBrowserWindowManager:
			wm, err := plugin.WindowManager(g.Token, e.broker)
			if err != nil {
				log.Warnf("Could not acquire browser window manager: %v. "+
					"Will not be able to create reports.", err)
				continue
			}
			e.wm = wm
		case plugin.PermissionBrowserResourceOpener:
			o, err := plugin.ResourceOpener(g.Token, e.broker)
			if err != nil {
				log.Warnf("Could not acquire browser resource opener: %v. "+
					"Will not be able to create reports.", err)
				continue
			}
			e.o = o
		case plugin.PermissionBrowserMessenger:
			m, err := plugin.Messenger(g.Token, e.broker)
			if err != nil {
				log.Warnf("Could not acquire browser messenger: %v. "+
					"Will not be able to report errors to user.", err)
				continue
			}
			e.m = m
		case plugin.PermissionEditor:
			ed, err := plugin.Editor(g.Token, e.broker)
			if err != nil {
				log.Warnf("Could not acquire editor to subscribe command: %v."+
					" Will not be able to create reports", err)
				continue
			}
			err = ed.SubscribeEditorEvents(editorEvents, e)
			if err != nil {
				log.Warnf("Could not subscribe to editor events: %v. "+
					"Will not be able to create reports", err)
				continue
			}
			for cmd, fn := range e.cmds {
				cmd := cmd
				fn := fn
				err = ed.SubscribeCommand(cmd, text.FuncCommandHandler(
					func(ctx context.Context, cmd text.Command) (bool, error) {
						return fn(e, ctx, cmd)
					}))
				if err != nil {
					log.Warnf("Could not subscribe command %q: %v", cmd, err)
					continue
				}
				log.Infof("Subscribed to create issue command %q", cmd)
			}
		case plugin.PermissionSchemeManager:
			m, err := plugin.SchemeManager(g.Token, e.broker)
			if err != nil {
				log.Warnf("Could not acquire scheme manager: %v. "+
					"Will not be able to create or see reports", err)
				continue
			}
			e.sm = m
		case plugin.PermissionStorage:
			s, err := plugin.Storage(g.Token, e.broker)
			if err != nil {
				log.Warnf("Could not acquire storage: %v. "+
					"Will not be able to create or see reports", err)
				continue
			}
			e.s = s
		}
	}

	if e.sm == nil || e.s == nil {
		log.Fatalf("Shutting down: Could not acquire storage or scheme manager")
	}

	cfg, err := sourceConfig(e.bluectlConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	svc, err := firestore.New(cfg.Auth.ProjectID,
		cfg.Issue.Collection, cfg.Auth.CredentialsFile)
	if err != nil {
		log.Fatalf("firestore: %v", err)
	}

	svc = logging.WithLogging(svc)

	// avoid too many reads to firestore, which is pretty slow.
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
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, initialEvictAllTimeout)
	defer cancel()

	err = e.svc.EvictAll(ctx)
	if err != nil {
		log.Warnf("Could not initialize issue cache: %v", err)
		return
	}
	log.Debugf("initialized plugin_issues successfully")
}

func (e *issuesGrantee) PermissionDenied(perms []plugin.Permission) {
	log.Fatalf("Could not start plugin due to missing permissions: "+
		"denied: %v; required: %v", perms, requiredPermissions)
}

func (e *issuesGrantee) Health() error {
	return nil
}

func (e *issuesGrantee) Shutdown(reason string) error {
	log.Debugf("plugin being shutdown: %s", reason)
	return nil
}

func (e *issuesGrantee) setMessage(msg string, args ...any) {
	if e.m == nil {
		return
	}
	err := e.m.SetMessage(msg, args...)
	if err != nil {
		err = fmt.Errorf("set message: %v", err)
		log.Error(err)
	}
}

func (e *issuesGrantee) issueRefresh(ctx context.Context, cmd text.Command) (bool, error) {
	if e.svc == nil {
		return false, errors.New("cannot refresh issues if permissions were not granted")
	}
	return false, e.svc.EvictAll(ctx)
}

func (e *issuesGrantee) openEmptyIssueTemplate(ctx context.Context, cmd text.Command) (bool, error) {
	return e.openIssueTemplate(ctx, e.defTemplate, "issue-")
}

func (e *issuesGrantee) openCustomIssueTemplate(
	template []byte, templateName string,
) func(*issuesGrantee, context.Context, text.Command) (bool, error) {
	return func(e *issuesGrantee, ctx context.Context, cmd text.Command) (bool, error) {
		return e.openIssueTemplate(ctx, template, templateName)
	}
}

func (e *issuesGrantee) freeIssue(ctx context.Context, ev text.Event) bool {
	if e.pendingIssueID == "" {
		e.setMessage("canceled creation of new issue")
	}
	_ = os.Remove(e.pendingIssueURI.Path())
	e.pendingIssueURI = workspace.URI{}
	e.pendingIssueID = ""
	return false
}

func (e *issuesGrantee) createReport(ctx context.Context, temp issue.Report) string {
	id, err := e.tracker.CreateReport(ctx, temp)
	if err != nil {
		err = fmt.Errorf("create report: %v", err)
		e.setMessage(err.Error())
		log.Error(err)
		return ""
	}
	msg := fmt.Sprintf("created issue report %s", id)
	e.setMessage(msg)
	log.Info(msg)
	return id
}

func (e *issuesGrantee) updateReport(ctx context.Context, id string, temp issue.Report) {
	err := e.tracker.UpdateReport(ctx, id, temp)
	if err != nil {
		err = fmt.Errorf("update report: %v", err)
		e.setMessage(err.Error())
		log.Error(err)
		return
	}

	msg := fmt.Sprintf("updated issue report %s", id)
	e.setMessage(msg)
	log.Info(msg)
}

func (e *issuesGrantee) createOrUpdateIssue(ctx context.Context, ev text.Event) bool {
	if e.tracker == nil {
		log.Debugf("Cannot create or update issue if permissions were not granted")
		return false
	}
	var temp issue.Report
	err := e.marshaler.Unmarshal([]byte(ev.Content), &temp)
	if err != nil {
		err = fmt.Errorf("unmarshal: %v", err)
		e.setMessage(err.Error())
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
	ctx context.Context, template []byte, templateName string,
) (bool, error) {
	if e.o == nil || e.wm == nil {
		return false, errors.New("browser permissions necessary to create an issue were not granted")
	}
	if !e.pendingIssueURI.Equal(workspace.URI{}) {
		return false, errors.New("there's already a pending issue open. " +
			"You should close it first before attempting to create a new one.")
	}

	focusWin, err := e.wm.Focus()
	if err != nil {
		return false, fmt.Errorf("get window in focus: %v", err)
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
	uri, err := workspace.CurrentUserHostURI(f.Name())
	if err != nil {
		return false, fmt.Errorf("URI: %v", err)
	}

	h, err := e.o.Open(uri)
	if err != nil {
		_ = os.Remove(f.Name())
		return false, fmt.Errorf("open temp file: %v", err)
	}

	err = focusWin.SetContent(h)
	if err != nil && !errors.Is(err, browser.ErrTabNotFree) {
		_ = h.Close()
		_ = os.Remove(f.Name())
		return false, fmt.Errorf("set focus window content: %v", err)
	}

	e.pendingIssueURI = uri

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

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:4568", nil))
	}()

	m := yaml.Marshaler()
	defaultAuthor := getDefaultAuthor()
	defTemplate := issue.Report{Author: defaultAuthor}
	data, err := m.Marshal(defTemplate)
	if err != nil {
		log.Fatalf("Marshal %v", err)
	}

	s := issuesGrantee{
		cmds:        defaultCommands,
		marshaler:   m,
		defTemplate: data,
	}
	plugin.Serve(&s, requiredPermissions...)
}
