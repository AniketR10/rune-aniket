// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package apiclient

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/url"
	"sync/atomic"

	"github.com/ernestrc/go-multierror"
	"github.com/ernestrc/sensible/browser"
	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/logging/trace"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"github.com/unstablebuild/ox-api/api"
	"github.com/unstablebuild/ox-api/auth"
	"unstable.build/go-tui/debug"
)

const (
	doneCopy = `
<script type="text/javascript">
</script>
Success! Please close this tab.
`
)

// Client implements a client to an instance of ox-api.
// This client should be subscribed to events as a text.EventHandler,
// for the telemetry implementation to collect all stats.
type Client struct {
	config               Config
	dataDir              string
	httpEndpointURL      *url.URL
	notifications        browserapi.Notifications
	tokenSource          *auth.CachedTokenSource
	storage              storageapi.Service
	ctx                  context.Context
	ctxCancel            func()
	isLogin              atomic.Bool
	telemetry            *telemetry
	telemetryTokenSource *auth.CachedTokenSource
}

// New returns allocates storage for a new Client and initializes it.
func New(
	n browserapi.Notifications, storage storageapi.Service,
	config Config, dataDir string,
) (*Client, error) {
	httpEndpointURL, err := url.Parse(config.HTTPEndpointAddress)
	if err != nil {
		return nil, fmt.Errorf("parse http endpoint url: %w", err)
	}
	ret := &Client{
		config:          config,
		dataDir:         dataDir,
		httpEndpointURL: httpEndpointURL,
		notifications:   n,
	}
	authStorage := storageapi.WithPartition(storage, "auth")
	ret.storage = authStorage
	ret.tokenSource = auth.NewCachedTokenSource(ret, authStorage, n)
	ret.ctx, ret.ctxCancel = context.WithCancel(context.Background())

	if config.EnableTelemetry {
		// for telemetry we only want to use the cached token, if there's any
		// or refresh token
		refreshOnlySourcer := auth.FuncTokenSourcer(
			func(ctx context.Context, t *oauth2.Token) (oauth2.TokenSource, error) {
				return ret.tokenSourceRefresh(ctx, t, true)
			})
		ret.telemetryTokenSource = auth.NewCachedTokenSource(refreshOnlySourcer, authStorage, n)
		ret.telemetry = newTelemetry(ret.telemetryTokenSource,
			ret.httpEndpointURL, ret.config.TelemetryPeriod, debug.Tag)
		ret.telemetry.start()
	}

	return ret, nil
}

// Handle satisfies text.EventHandler.
func (t *Client) Handle(ctx context.Context, ev textapi.Event) bool {
	if t.telemetry == nil {
		return false
	}
	return t.telemetry.Handle(ctx, ev)
}

// TelemetryEnabled returns whether telemetry is active for this client.
func (a *Client) TelemetryEnabled() bool {
	return a.telemetry != nil
}

// TokenSource satisfies auth.TokenSourcer.
func (a *Client) TokenSource(ctx context.Context, token *oauth2.Token) (
	oauth2.TokenSource, error,
) {
	return a.tokenSourceRefresh(ctx, token, false)
}

// OAuthTokenSource returns the underlying oauth2.TokenSource used for
// authenticated requests. This can be used to build an *http.Client via
// oauth2.NewClient for HTTP-based APIs such as cdnrelease.
func (a *Client) OAuthTokenSource() oauth2.TokenSource {
	return a.tokenSource
}

// Logout purges the user's underlying authentication credentials,
// so next requests sent to the server via the connections created via NewConn
// will be un-authenticated.
func (a *Client) Logout(ctx context.Context) error {
	if err := a.tokenSource.Purge(); err != nil {
		log.Warnf("cache reset: %v", err)
	}

	// NOTE: we do not want to purge telemetry token source, as it doesn't
	// have effect on the user functionality, but we want to maintain authed logs
	// _ = a.telemetryTokenSource.Purge()

	if _, err := a.notifications.Notify(browserapi.LevelSuccess,
		"You have been successfully logged out."); err != nil {
		log.Errorf("set message: %v", err)
	}
	return nil
}

// Login authenticates the user using a browser oauth2 flow. After this flow
// is successful, requests sent through the connections returned by NewConn will
// carry the credentials acquired during this flow.
func (a *Client) Login(ctx context.Context) error {
	// perform async to avoid blocking event loop while
	// we are waiting for oauth2 browser flow.
	go debug.CapturePanicReport(func() {
		a.isLogin.Store(true)
		defer a.isLogin.Store(false)
		// force source to acquire new token
		_, err := a.tokenSource.Token()
		if err != nil {
			log.Warnf("login: %v", err)
			_, err := a.notifications.Notify(browserapi.LevelError, "%v", err)
			if err != nil {
				log.Errorf("set message: %v", err)
			}
			return
		}
		if _, err := a.notifications.Notify(browserapi.LevelSuccess,
			"You are successfully logged in."); err != nil {
			log.Errorf("set message: %v", err)
		}
	})

	return nil
}

// Dial creates a new grpc.ClientConn that uses the underlying
// user authentication state to send authenticated or unauthenticated requests.
// The returned grpc.ClientConn's lifecycle is responsibility of the caller, i.e.
// grpc.ClientConn.Close is not called when Client.Close is called.
func (a *Client) Dial() (*grpc.ClientConn, error) {
	rpcCreds := oauth.TokenSource{TokenSource: a.tokenSource}
	opts := []grpc.DialOption{
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(api.MaxRecvMsgSize),
			grpc.MaxCallRecvMsgSize(api.MaxSendMsgSize),
		),
	}

	if a.config.InsecureTransport {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		certPool, err := x509.SystemCertPool()
		if err != nil {
			err = fmt.Errorf("x509 system cert pool: %v", err)
			return nil, err
		}
		transportCreds := credentials.NewClientTLSFromCert(certPool, "")
		opts = append(opts, grpc.WithTransportCredentials(transportCreds))
		opts = append(opts, grpc.WithPerRPCCredentials(rpcCreds))
	}

	conn, err := grpc.NewClient(a.config.GRPCEndpointAddress, opts...)
	if err != nil {
		err = fmt.Errorf("dial rune GRPC address: %v", err)
		return nil, err
	}
	return conn, nil
}

// Close closes all resources associated with this Client,
// except connections created via NewConn.
func (a *Client) Close() (ret error) {
	if a.telemetry != nil {
		if err := a.telemetry.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	if a.storage != nil {
		if err := a.storage.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	a.ctxCancel()
	return
}

var (
	tryPorts = []int{
		11524, 12524, 12624,
		21524, 22524, 22624,
		31524, 32524, 32624,
		41524, 42524, 42624,
		51524, 52524, 52624,
		61524, 62524, 62624,
	}
)

func (a *Client) tokenSourceRefresh(ctx context.Context, token *oauth2.Token, refreshOnly bool) (
	oauth2.TokenSource, error,
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// propagate traceID to grantee logs
	traceID, ctx := trace.FromContextOrNew(ctx)
	log := log.WithFields(log.Fields{logging.KeyTraceID: traceID})

	conf, err := auth.FetchConfig(a.httpEndpointURL)
	if err != nil {
		log.Warnf("could not fetch oauth2 configuration, fallback to builtin: %v", err)
		conf = auth.DefaultNativeConfig(a.httpEndpointURL)
	}
	log.Infof("acquiring new oauth2 token source: "+
		"http=%v grpc=%v config_url=%v "+
		"auth_url=%v token_url=%v jwks_url=%v api_url=%v signup_url=%v "+
		"client_id=%v scopes=%v "+
		"token=%v valid=%v",
		a.config.HTTPEndpointAddress, a.config.GRPCEndpointAddress,
		a.httpEndpointURL.JoinPath(auth.ServeConfigPath),
		conf.Endpoint.AuthURL, conf.Endpoint.TokenURL, conf.JWKSURL,
		conf.APIURL, conf.SignupURL,
		conf.ClientID, conf.Scopes,
		token != nil, token.Valid())

	if token != nil {
		// use component lifecycle ctx rather than this rpc's ctx
		// to ensure that the refresh process is not canceled incorrectly
		source := conf.TokenSource(a.ctx, token)
		return source, nil
	}

	if refreshOnly {
		// signal that this is an expected state so it is not logged as an error
		return nil, auth.ErrUnavailable
	}

	var msg string
	if !a.isLogin.Load() {
		msg = "Need to login first before using the Rune API. " +
			"Please follow instructions in web browser"
	} else {
		msg = "Please follow instructions in web browser"
	}
	_, err = a.notifications.Notify(browserapi.LevelInfo, msg)
	if err != nil {
		return nil, fmt.Errorf("notify: %w", err)
	}

	log.Debugf("oauth2: waiting for oauth2 flow to complete")

	_, source, err := blueauth.NewClientWithPorts(ctx, conf.Config, func(rawURL string) error {
		u, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		if err := browser.Browse(u); err != nil {
			return fmt.Errorf("%v. "+
				"Make sure that $BROWSER environment variable is set correctly",
				err)
		}

		return nil
	}, doneCopy, tryPorts)
	if err != nil {
		return nil, fmt.Errorf("new oauth2 client: %w", err)
	}
	log.Debugf("oauth2: successfully generated token source")
	return source, nil
}
