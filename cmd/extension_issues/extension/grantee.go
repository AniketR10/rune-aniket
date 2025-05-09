// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package extension

import (
	"context"
	"errors"
	"fmt"
	_ "net/http/pprof"
	"os"
	"os/user"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/doclog"
	"github.com/unstablebuild/blue/document/docmarshal"
	"github.com/unstablebuild/blue/document/docmarshal/docyaml"
	"github.com/unstablebuild/blue/issue"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/browserapi/browserext"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/schemeapi"
	"unstable.build/go-tui/api/schemeapi/schemeext"
	"unstable.build/go-tui/api/storageapi/storageext"
	"unstable.build/go-tui/api/textapi"
	"unstable.build/go-tui/api/textapi/textext"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/localstorage/storagecache"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/workspace/docscheme"
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
			handler: (*Grantee).openEmptyIssueTemplate,
		},
		issueRefreshCmd: {
			man: textapi.CommandManual{
				Summary: "Refreshes the local issues cache. This is useful when user " +
					"knows that out-of-band changes have been made to the issue tracker.",
			},
			handler: (*Grantee).issueRefresh,
		},
	}
	editorEvents = []textapi.EventType{textapi.EventTypeFlush, textapi.EventTypeClose}
)

// GranteeWithService returns this extension's Grantee and the permissions required to run it.
func GranteeWithService(
	versionTag, scheme string,
	svcFn func(config.Config) (document.Service, error),
) (extension.Grantee, []extension.Permission) {
	s := NewGrantee(versionTag, scheme, svcFn)
	return s, requiredPermissions
}

// Grantee returns an issues extension grantee that adds the ability to create
// issues from templates by running commands, and optionally adds an issues scheme.
type Grantee struct {
	config     config.Config
	broker     rpc.MuxBroker
	svc        *storagecache.Service[issue.ReportDocument]
	tracker    issue.Tracker
	marshaler  docmarshal.Marshaler
	m          browserapi.Notifications
	o          browserapi.ResourceOpener
	wm         browserapi.WindowManager
	sm         schemeapi.SchemeManager
	s          document.Service
	versionTag string
	svcFn      func(config.Config) (document.Service, error)
	scheme     string

	cmds            map[string]commandAll
	maxSubjectLen   int
	registerScheme  bool
	defTemplate     []byte
	disableCommands bool
	author          string

	pendingIssueID  string       // accessed by Handle only, no need to synchronize
	pendingIssueURI atomic.Value // workspaceapi.URI, accessed by Handle and HandleCommand
}

// NewGrantee allocates storage for a new Grantee and initializes it.
func NewGrantee(
	versionTag, scheme string,
	svcFn func(config.Config) (document.Service, error),
) *Grantee {
	m := docyaml.Marshaler()
	s := &Grantee{
		cmds:       defaultCommands,
		marshaler:  m,
		versionTag: versionTag,
		scheme:     scheme,
		svcFn:      svcFn,
	}
	s.pendingIssueURI.Store(workspaceapi.URI{})
	return s
}

// Connected satisfies extension.Grantee.
func (e *Grantee) Connected(
	ctx context.Context, broker rpc.MuxBroker, pconfig config.Config,
) (err error) {
	e.config = pconfig
	e.broker = broker

	e.author, err = pconfig.GetString("author")
	if err != nil {
		if err != config.ErrNotFound {
			return fmt.Errorf("could not read property "+
				"'author': %w", err)
		}
		e.author = getDefaultAuthor()
	}

	defTemplate := issue.Report{Author: e.author}
	data, err := e.marshaler.Marshal(defTemplate)
	if err != nil {
		return fmt.Errorf("marshal default issue template: %w", err)
	}
	e.defTemplate = data

	e.disableCommands, err = pconfig.GetBool("disable_commands")
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

	e.registerScheme, err = pconfig.GetBool("register_scheme")
	if err != nil {
		if err != config.ErrNotFound {
			return fmt.Errorf("could not read property "+
				"'register_scheme': %w", err)
		}
		e.registerScheme = true
	}

	templatesMap, err := pconfig.GetMap("templates")
	if err != nil {
		if err != config.ErrNotFound {
			return fmt.Errorf("could not read property 'templates': %w", err)
		}
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

	if e.disableCommands {
		e.cmds = make(map[string]commandAll)
		return nil
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

// OpenIssueTemplate opens an issue template as a new temporary tab, and awaits
// for the user to flush to disk to synchronize it to the issue tracker.
// The given context can override the default author
func (e *Grantee) OpenIssueTemplate(
	ctx context.Context, template issue.Report, cmd textapi.Command,
) error {
	data, err := e.marshaler.Marshal(template)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}
	err = e.openIssueTemplate(ctx, cmd.Window, data, "custom-create-template")
	return err
}

// Handle satisfies text.EventHandler.
func (e *Grantee) Handle(ctx context.Context, ev textapi.Event) bool {
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

// PermissionGranted satisfies extension.Grantee.
func (e *Grantee) PermissionGranted(ctx context.Context, grants []extension.Grant) error {
	e.log(log.DebugLevel, "permissions granted: %v", grants)

	for _, g := range grants {
		switch g.Permission {
		case extension.Permission(extension.PermissionBrowserWindowManager):
			wm, err := browserext.WindowManager(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire browser window manager: %w ", err)
			}
			e.wm = wm
		case extension.Permission(extension.PermissionBrowserResourceOpener):
			o, err := browserext.ResourceOpener(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire browser resource opener: %w ", err)
			}
			e.o = o
		case extension.Permission(extension.PermissionBrowserNotifications):
			m, err := browserext.Notifications(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire browser notifications: %w ", err)
			}
			e.m = m
		case extension.Permission(extension.PermissionEditor):
			ed, err := textext.Editor(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire editor: %w ", err)
			}
			err = ed.SubscribeEvents(editorEvents, e)
			if err != nil {
				return fmt.Errorf("subscribe editor events: %w ", err)
			}
			if e.disableCommands {
				continue
			}
			for cmd, man := range e.cmds {
				cmd := cmd
				man := man
				man.man.Name = cmd
				err = ed.SubscribeCommand(man.man, textapi.NopCommandCompleter(
					func(ctx context.Context, cmd textapi.Command) error {
						return man.handler(e, ctx, cmd)
					}))
				if err != nil {
					return fmt.Errorf("subscribe command %q: %w", cmd, err)
				}
				e.log(log.DebugLevel, "Subscribed to create issue command %q", cmd)
			}
		case extension.PermissionSchemeManager:
			m, err := schemeext.SchemeManager(ctx, g, e.broker)
			if err != nil {
				return fmt.Errorf("acquire scheme manager: %w", err)
			}
			e.sm = m
		case extension.PermissionStorage:
			s, err := storageext.Storage(ctx, g, e.broker)
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
	svc = doclog.WithLogging(svc, "IssuesStorage")

	// avoid too many reads to service (i.e. firestore), which is pretty slow.
	// As long as there aren't many oob (outside of six) requests this should be
	// able to cache expensive list + get all operations pretty well
	// because it's using the host's global storage as the cache
	e.svc = storagecache.New[issue.ReportDocument](svc, e.s)
	e.tracker = issue.NewDocumentTracker(e.svc)

	if !e.registerScheme {
		return nil
	}

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

// PermissionDenied satisfies extension.Grantee.
func (e *Grantee) PermissionDenied(ctx context.Context, perms []extension.Permission) error {
	return fmt.Errorf("missing critical permissions: denied: %v; required: %v",
		perms, requiredPermissions)
}

// Health satisfies extension.Grantee.
func (e *Grantee) Health(context.Context) error {
	return nil
}

// Shutdown satisfies extension.Grantee.
func (e *Grantee) Shutdown(ctx context.Context, reason string) error {
	e.log(log.DebugLevel, "shutdown: %s", reason)
	return nil
}

func (e *Grantee) notify(level notifications.Level, msg string, args ...any) {
	err := e.m.Notify(level, msg, args...)
	if err != nil {
		e.log(log.ErrorLevel, "notify: %v", err)
	}
}

func (e *Grantee) issueRefresh(ctx context.Context, cmd textapi.Command) error {
	return e.svc.EvictAll(ctx)
}

func (e *Grantee) openEmptyIssueTemplate(ctx context.Context, cmd textapi.Command) error {
	return e.openIssueTemplate(ctx, cmd.Window, e.defTemplate, "issue-")
}

func (e *Grantee) openCustomIssueTemplate(
	template []byte, templateName string,
) func(*Grantee, context.Context, textapi.Command) error {
	return func(e *Grantee, ctx context.Context, cmd textapi.Command) error {
		return e.openIssueTemplate(ctx, cmd.Window, template, templateName)
	}
}

func (e *Grantee) freeIssue(ctx context.Context, ev textapi.Event, uri workspaceapi.URI) bool {
	if e.pendingIssueID == "" {
		e.notify(notifications.LevelInfo, "canceled creation of new issue")
	}
	e.pendingIssueURI.CompareAndSwap(uri, workspaceapi.URI{})
	_ = os.Remove(uri.Path())
	e.pendingIssueID = ""
	return false
}

func (e *Grantee) createReport(ctx context.Context, temp issue.Report) string {
	id, err := e.tracker.CreateReport(ctx, temp)
	if err != nil {
		err = fmt.Errorf("create report: %w", err)
		e.notify(notifications.LevelError, err.Error())
		e.log(log.ErrorLevel, "%s", err.Error())
		return ""
	}
	msg := fmt.Sprintf("created issue report %s", id)
	e.notify(notifications.LevelSuccess, msg)
	e.log(log.InfoLevel, "%s", msg)
	return id
}

func (e *Grantee) updateReport(ctx context.Context, id string, temp issue.Report) {
	err := e.tracker.UpdateReport(ctx, id, temp)
	if err != nil {
		err = fmt.Errorf("update report: %v", err)
		e.notify(notifications.LevelError, err.Error())
		e.log(log.ErrorLevel, "%s", err.Error())
		return
	}

	msg := fmt.Sprintf("updated issue report %s", id)
	e.notify(notifications.LevelSuccess, msg)
	e.log(log.InfoLevel, "%s", msg)
}

func (e *Grantee) createOrUpdateIssue(ctx context.Context, ev textapi.Event) bool {
	var temp issue.Report
	err := e.marshaler.Unmarshal([]byte(ev.Content), &temp)
	if err != nil {
		err = fmt.Errorf("unmarshal: %v", err)
		e.notify(notifications.LevelError, err.Error())
		e.log(log.WarnLevel, "%s", err.Error())
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

func (e *Grantee) openIssueTemplate(
	ctx context.Context, win browserapi.Window, template []byte, templateName string,
) error {
	if e.o == nil || e.wm == nil {
		return errors.New("browser permissions necessary to create an issue were not granted")
	}
	oldURI := e.pendingIssueURI.Load().(workspaceapi.URI)
	if !oldURI.Equal(workspaceapi.URI{}) {
		return errors.New("there's already a pending issue open. " +
			"You should close it first before attempting to create a new one.")
	}

	f, err := os.CreateTemp("", templateName)
	if err != nil {
		return fmt.Errorf("temp file: %v", err)
	}

	_, err = f.Write(template)
	if err != nil {
		return fmt.Errorf("write template to file: %v", err)
	}

	_ = f.Close()
	newURI, err := workspaceapi.CurrentUserHostURI(f.Name())
	if err != nil {
		return fmt.Errorf("URI: %v", err)
	}

	h, err := e.o.Open(newURI)
	if err != nil {
		_ = os.Remove(f.Name())
		return fmt.Errorf("open temp file: %v", err)
	}

	err = win.SetContent(h)
	if err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		_ = h.Close()
		_ = os.Remove(f.Name())
		return fmt.Errorf("set focus window content: %v", err)
	}

	e.pendingIssueURI.Store(newURI)

	return nil
}

func (e *Grantee) initScheme(m schemeapi.SchemeManager) error {
	marshaler := docyaml.Marshaler()
	rootURI, err := workspaceapi.ParseURI(fmt.Sprintf("%s:///", e.scheme))
	if err != nil {
		panic(err)
	}

	schemeFn := docscheme.Scheme[issue.ReportDocument](rootURI, e.svc,
		marshaler, fmt.Errorf("missing %q sub-field in Metadata field",
			issue.ReportMetadataIDField), e.author)
	schemeFn = issueMapperScheme(schemeFn, marshaler, e.maxSubjectLen)
	err = m.RegisterScheme(e.scheme, schemeFn)
	if err == schemeapi.ErrSchemeAlreadyRegistered {
		// the first workspace to run this plugin registers the scheme successfully
		err = nil
		e.log(log.DebugLevel, "ignore register error: another issues extension registered the scheme first")
	}
	return err
}

func (t *Grantee) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "issuesextension.Grantee",
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
	handler func(*Grantee, context.Context, textapi.Command) error
}
