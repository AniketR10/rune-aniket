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

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/document/firestore"
	"github.com/ernestrc/blue/encoding"
	"github.com/ernestrc/blue/encoding/yaml"
	"github.com/ernestrc/blue/issue"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
	workspacedoc "unstable.build/go-tui/workspace/document"
)

const (
	issuesScheme         = "bluectl+issues"
	defaultMaxSubjectLen = 50
	defaultCommand       = "issueCreate"
)

var (
	// compile-time variable
	Tag = "development"

	requiredPermissions = []plugin.Permission{
		plugin.PermissionBrowserWindowManager,
		plugin.PermissionBrowserMessenger,
		plugin.PermissionBrowserResourceOpener,
		plugin.PermissionSchemeManager,
		plugin.PermissionEditor,
	}
	defaultCommands = map[string]func(*issuesGrantee,
		context.Context, text.Command) (bool, error){
		defaultCommand: (*issuesGrantee).openEmptyIssueTemplate,
	}
	editorEvents = []text.EventType{text.EventTypeFlush, text.EventTypeClose}
)

type issuesGrantee struct {
	mu        sync.Mutex
	broker    proto.MuxBroker
	svc       document.Service
	tracker   issue.Tracker
	marshaler encoding.Marshaler
	m         browser.Messenger
	o         browser.ResourceOpener
	wm        browser.WindowManager

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
	cfg, err := sourceConfig(e.bluectlConfigFile)
	if err != nil {
		log.Fatal(err)
	}
	svc, err := firestore.New(cfg.Auth.ProjectID,
		cfg.Issue.Collection, cfg.Auth.CredentialsFile)
	if err != nil {
		log.Fatal(err)
	}
	e.svc = svc
	e.tracker = issue.NewDocumentTracker(e.svc)

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

func (e *issuesGrantee) initScheme(g plugin.Grant) error {
	m, err := plugin.SchemeManager(g.Token, e.broker)
	if err != nil {
		return err
	}

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
			err := e.initScheme(g)
			if err != nil {
				log.Warnf("Could not register scheme: %v", err)
			}
		}
	}
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

func (e *issuesGrantee) openEmptyIssueTemplate(ctx context.Context, cmd text.Command) (bool, error) {
	return e.openIssueTemplate(ctx, e.defTemplate, "new-issue-report")
}

func (e *issuesGrantee) openCustomIssueTemplate(
	template []byte, templateName string,
) func(*issuesGrantee, context.Context, text.Command) (bool, error) {
	return func(e *issuesGrantee, ctx context.Context, cmd text.Command) (bool, error) {
		return e.openIssueTemplate(ctx, template, templateName)
	}
}

func (e *issuesGrantee) freeIssue(ctx context.Context, ev text.Event) bool {
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
	if err != nil && !errors.Is(browser.ErrTabNotFree, err) {
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
