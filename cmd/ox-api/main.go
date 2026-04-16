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

package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"cloud.google.com/go/storage"
	log "github.com/sirupsen/logrus"
	blueauth "github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/blue/auth/grpcauth"
	"github.com/unstablebuild/blue/auth/secretmanager"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/firestore"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/blue/release/docrelease"
	"github.com/unstablebuild/blue/release/gcsrelease"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"unstable.build/go-tui/cmd/ox-api/oxapi"
	"unstable.build/go-tui/cmd/rune/api"
	"unstable.build/go-tui/cmd/rune/auth"
)

const (
	signKeySecretID    = "oauth2_keys"
	auth0SecretID      = "auth0-ox-api-prod"
	serverCertSecretID = "grpc_hopperapi_cert"
	serverKeySecretID  = "grpc_hopperapi_key"
)

var (
	// Tag is a compile-time variable.
	Tag = "development"
	// Commit is a compile-time variable.
	Commit = "HEAD"
	// Version is a combination of Tag and Commit, representing
	// this executables version.
	Version string

	authConfig = auth.DefaultConfig()

	version          = flag.Bool("v", false, "Print version information to stdout")
	verbose          = flag.Bool("V", false, "Enable verbose logging")
	jsonLogFormatter = flag.Bool("J", false, "Enable JSON log formatter for structured logs.")
	httpPort         = flag.Int("P", 3001, "Listening HTTP port")
	grpcPort         = flag.Int("G", 4001, "Listening GRCP port")
	host             = flag.String("H", "127.0.0.1", "Listening HTTP/GRPC address")
	signupURLStr     = flag.String("U", authConfig.SignupURL, "Signup URL to redirect users to")
	apiURLStr        = flag.String("A", "https://api.unstable.build", "API URL of this listening server")
	// optional because in GCP the creds are passed to SDK
	// constructors via environment variable.
	gcCredsFile = flag.String("c", "",
		"Google Cloud credentials file for datastore")
	gcProjectID       = flag.String("p", "", "Google Cloud project ID")
	gcPubJWKSEndpoint = flag.String("k", authConfig.JWKSURL,
		"Google Cloud public JWKS http endpoint. If empty, then no Google "+
			"JWKS is added to the verfiy chain but this is a failure mode that should only be "+
			"employed in case the google certs endpoint is hard down and we are unable to boot an ox-api service "+
			"and so it's the only way to keep the rune rpc users online. Website will not be functional.")

	issueCollection       = flag.String("i", "blue-issues-beta", "Firestore collection for managing issues")
	accountCollection     = flag.String("a", "rune-account", "Firestore collection for managing account")
	releaseCollectionBase = flag.String("r", "rune-release", "Firestore base collection for managing packages; the runtime architecture suffix is appended automatically")
	gcsBucketBase         = flag.String("b", "rune-release", "GCS base bucket for release binary storage; the runtime architecture suffix is appended automatically")

	reportBucket          = flag.String("report-bucket", "rune-reports", "GCS bucket for report storage")
	reportPrefix          = flag.String("report-prefix", "reports/", "Object name prefix within the report GCS bucket")
	reportMaxBytes        = flag.Int64("report-max-bytes", int64(oxapi.DefaultReportMaxBytes), "Maximum report payload size in bytes")
	reportRateLimitWindow = flag.Duration("report-rate-limit-window", oxapi.DefaultReportRateLimitWindow, "Time window for bucketing duplicate report fingerprints")

	// refreshCredsEvery = flag.Duration("R", time.Hour, "Cadence at which to refresh GRPC transport credentials")
	tlsCert     = flag.String("C", "", "TLS certificate used for grpc credentials.")
	tlsKey      = flag.String("K", "", "TLS key used for grpc credentials.")
	tlsInsecure = flag.Bool("I", false, "Do not setup GRPC API with TLS.")
	grpcNoAuth  = flag.Bool("X", false, "Do not setup GRPC API with authentication.")

	tokenExpiry = flag.Duration("E", 30*time.Minute, "Oauth2 access token expiry.")
)

func init() {
	Version = fmt.Sprintf("%s (HEAD is %s)", Tag, Commit)
}

func parseFlags() {
	flag.Parse()

	if *version {
		fmt.Printf("Rune API Server (%s)\n", Version)
		os.Exit(0)
	}

	logging.SetDefaults(*verbose)
	if *jsonLogFormatter {
		log.SetFormatter(logging.LogrusGCPFormatter{})
	}

	if *gcProjectID == "" {
		log.Fatal("Must pass -p flag")
	}
	if *tlsCert == "" && !*tlsInsecure {
		log.Fatal("Must pass -C and -K, or -I flag")
	}
	if *tlsKey == "" && !*tlsInsecure {
		log.Fatal("Must pass -C and -K, or -I flag")
	}
}

func validateReleaseSigning(ctx context.Context, signer gcsrelease.Signer) error {
	_, err := signer.SignedDownloadURL(ctx, "signing-probe", release.Version("0"))
	if err != nil {
		return fmt.Errorf("validate release signing: %w", err)
	}
	return nil
}

func main() {
	parseFlags()
	var err error
	var accountDB document.Service

	accountDB, err = firestore.New(*gcProjectID,
		*accountCollection, *gcCredsFile)
	if err != nil {
		log.Fatalf("firestore new: %v", err)
	}

	signKeys, err := secretmanager.SymmetricKeys(*gcProjectID, *gcCredsFile, signKeySecretID)
	if err != nil {
		log.Fatalf("secretmanager keys: %v", err)
	}

	verifyKeys := signKeys

	if urlStr := *gcPubJWKSEndpoint; urlStr != "" {
		url, err := url.Parse(urlStr)
		if err != nil {
			log.Fatalf("parse -k flag: %v", err)
		}
		googleKeys, err := blueauth.FetchPublicJWKS(url)
		if err != nil {
			log.Fatalf("fetch google public JWKS: %v", err)
		}
		verifyKeys = blueauth.CombineKeys(verifyKeys, googleKeys)
		authConfig.JWKSURL = url.String()
	}

	// cache keys. In order to pick up new keys the service must
	// be restarted. If this is not good enough at some point
	// we should limit secretmanager list versions API calls.
	// which currently occur for every attempt to verify.
	signKeys = auth.KeysCache(signKeys)
	verifyKeys = auth.KeysCache(verifyKeys)

	// Build per-architecture release collection names from the configurable base.
	releaseCollections := make([]string, len(oxapi.SupportedArchitectures))
	for i, arch := range oxapi.SupportedArchitectures {
		releaseCollections[i] = fmt.Sprintf("%s-%s", *releaseCollectionBase, arch)
	}

	rpcApiAuthorizer := oxapi.RPCAuthorizer(*issueCollection, releaseCollections)

	// Setup per-architecture release managers (Firestore metadata + GCS binary storage).
	gcsClient, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("gcs client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	archReleases := make([]oxapi.ArchRelease, len(oxapi.SupportedArchitectures))
	for i, arch := range oxapi.SupportedArchitectures {
		collection := releaseCollections[i]
		bucketName := fmt.Sprintf("%s-%s", *gcsBucketBase, arch)

		releaseDB, err := firestore.New(*gcProjectID, collection, *gcCredsFile)
		if err != nil {
			log.Fatalf("release firestore new (%s): %v", arch, err)
		}
		inner := docrelease.NewManager(releaseDB)
		bucket := gcsClient.Bucket(bucketName)
		mgr := gcsrelease.NewManager(inner, gcsrelease.NewBucket(bucket))

		if err := validateReleaseSigning(ctx, mgr); err != nil {
			log.Fatalf("release signing (%s): %v", arch, err)
		}

		archReleases[i] = oxapi.ArchRelease{
			Arch:    arch,
			Manager: mgr,
			Signer:  mgr,
		}
	}

	secretStore, err := secretmanager.SecretStore(*gcProjectID, *gcCredsFile)
	if err != nil {
		log.Fatalf("secretmanager secret store: %v", err)
	}

	// Use letsencrypt key/cert for now.
	//creds, err := bluecreds.ServerTransport(*gcProjectID, *gcCredsFile,
	//	serverCertSecretID, serverKeySecretID, *refreshCredsEvery)
	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(api.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(api.MaxSendMsgSize),
	}
	if *tlsInsecure && !*grpcNoAuth {
		opts = append(opts, grpcauth.GRPCServerWithInsecureOauth2(verifyKeys, rpcApiAuthorizer)...)
	} else if !*tlsInsecure {
		creds, err := credentials.NewServerTLSFromFile(*tlsCert, *tlsKey)
		if err != nil {
			log.Fatalf("grpc credentials: %v", err)
		}
		if *grpcNoAuth {
			opts = append(opts, grpc.Creds(creds))
		} else {
			opts = append(opts, grpcauth.GRPCServerWithOauth2(verifyKeys, rpcApiAuthorizer, creds)...)
		}
	}
	srv := grpc.NewServer(opts...)
	if err := registerGRPCApi(*gcProjectID, *gcCredsFile,
		srv, *issueCollection, releaseCollections); err != nil {
		log.Fatalf("grpc register: %v", err)
	}

	grpcAddr := fmt.Sprintf("%s:%d", *host, *grpcPort)
	grpcLis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}

	go func() {
		log.Infof("GRPC server listening at address %s", grpcAddr)
		log.Fatal(srv.Serve(grpcLis))
	}()

	apiURL, err := url.Parse(*apiURLStr)
	if err != nil {
		log.Fatalf("url parse api url: %v", err)
	}

	signupURL, err := url.Parse(*signupURLStr)
	if err != nil {
		log.Fatalf("url parse signup url: %v", err)
	}
	authConfig.SignupURL = *signupURLStr

	// Setup report store (GCS-backed).
	reportBkt := gcsClient.Bucket(*reportBucket)
	var rptStore oxapi.ReportStore = oxapi.NewGCSReportStore(reportBkt)
	rptCfg := oxapi.ReportConfig{
		MaxBytes:        *reportMaxBytes,
		Prefix:          *reportPrefix,
		RateLimitWindow: *reportRateLimitWindow,
	}

	var httpHandler http.Handler
	httpHandler, err = oxapi.NewHTTPApi(accountDB, signKeys, verifyKeys,
		secretStore, *tokenExpiry, signupURL, apiURL, authConfig,
		archReleases, rpcApiAuthorizer,
		rptStore, rptCfg)
	if err != nil {
		log.Fatalf("http api: %v", err)
	}

	httpHandler = logging.NewMiddleware(httpHandler)

	httpAddr := fmt.Sprintf("%s:%d", *host, *httpPort)
	log.Infof("HTTP server (%s) listening at address %s", Version, httpAddr)
	log.Fatal(http.ListenAndServe(httpAddr, httpHandler))
}
