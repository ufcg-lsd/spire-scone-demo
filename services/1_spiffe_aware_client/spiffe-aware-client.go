package main

import (
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
	socketPath, ok := os.LookupEnv("AGENT_SOCKET_PATH")
	if !ok {
		log.Fatal("Error: socketPath undefined")
	}

	serverURL, ok := os.LookupEnv("SERVER_URL")
	if !ok {
		log.Fatal("Error: serverURL undefined")
	}

	serverTrustDomain, ok := os.LookupEnv("SERVER_TRUST_DOMAIN")
	if !ok {
		log.Fatal("Error: serverTrustDomain undefined")
	}

	serverSpiffeID, ok := os.LookupEnv("SERVER_SPIFFE_ID")
	if !ok {
		log.Fatal("Error: serverSpiffeID undefined")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a `workloadapi.X509Source`, it will connect to Workload API using provided socket path
	// If socket path is not defined using `workloadapi.SourceOption`, value from environment variable `SPIFFE_ENDPOINT_SOCKET` is used.
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
	if err != nil {
		log.Fatalf("Unable to create X509Source %v", err)
	}
	defer source.Close()

	// Allowed SPIFFE ID
	serverID := spiffeid.Must(serverTrustDomain, serverSpiffeID)

	// Create a `tls.Config` to allow mTLS connections, and verify that presented certificate has SPIFFE ID `spiffe://example.org/server`
	tlsConfig := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID))
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: time.Second * 10,
	}

	for {
		time.Sleep(10 * time.Second)
		log.Println("Requesting secret...")
		r, err := client.Get(serverURL)
		if err != nil {
			log.Printf("Error connecting to %q: %v\n---\n", serverURL, err)
			continue
		}

		defer r.Body.Close()
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("Unable to read body: %v\n---\n", err)
			continue
		}

		log.Printf("%s", body)
		log.Printf("\n---\n")
	}

}
