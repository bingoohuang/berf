package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/bingoohuang/berf/fgrpc/service"
	"log"
	"os"
	"strconv"

	"github.com/bingoohuang/berf"
	"github.com/bingoohuang/gg/pkg/ctl"
	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/shimingyah/pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	pVersion  = fla9.Bool("version", false, "Show version and exit")
	pServer   = fla9.String("server,s", "", "Server address")
	pTarget   = fla9.String("target", "status", "status/encrypt")
	pJSONFile = fla9.String("json", "", "json file")
	pInit     = fla9.Bool("init", false, "Create initial ctl and exit")
	pPool     = fla9.Bool("pool", false, "gRPC pool")
)

func init() {
	fla9.Parse()
	ctl.Config{Initing: *pInit, PrintVersion: *pVersion}.ProcessInit()
}

func main() {
	sigx.RegisterSignalProfile()
	b := &bench{}
	berf.StartBench(context.Background(), b, berf.WithOkStatus("200"))
}

type bench struct {
	conn           *grpc.ClientConn
	client         service.ServiceClient
	pool           pool.Pool
	encryptRequest service.EncryptRequest
}

func (b *bench) Name(ctx context.Context, config *berf.Config) string {
	return "grpc"
}

func (b *bench) Init(ctx context.Context, config *berf.Config) (*berf.BenchOption, error) {
	if *pPool {
		options := pool.DefaultOptions
		options.MaxActive = config.Goroutines
		p, err := pool.New(*pServer, options)
		if err != nil {
			return nil, fmt.Errorf("new pool: %w", err)
		}
		b.pool = p
	} else {
		// grpc uses HTTP 2 which is by default uses SSL
		// we use insecure (we can also use the credentials)
		conn, err := grpc.Dial(*pServer, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("could not connect to : %s: %w", *pServer, err)
		}
		b.conn = conn
		b.client = service.NewServiceClient(conn)
	}

	if *pTarget == "encrypt" {
		if *pJSONFile == "" {
			return nil, fmt.Errorf("json file required for encrypt")
		}

		jsonData, err := os.ReadFile(*pJSONFile)
		if err != nil {
			return nil, fmt.Errorf("read json file: %w", err)
		}

		if err := json.Unmarshal(jsonData, &b.encryptRequest); err != nil {
			return nil, fmt.Errorf("unmarshal json file: %w", err)
		}
	}

	return &berf.BenchOption{}, nil
}

func (b *bench) Invoke(ctx context.Context, config *berf.Config) (*berf.Result, error) {
	if *pPool {
		conn, err := b.pool.Get()
		if err != nil {
			return nil, fmt.Errorf("get conn: %w", err)
		}
		defer conn.Close()

		b.client = service.NewServiceClient(conn.Value())
	}

	switch *pTarget {
	case "encrypt":
		if config.N == 1 {
			log.Printf("request: %s", &b.encryptRequest)
		}
		rsp, err := b.client.Encrypt(ctx, &b.encryptRequest)
		if err != nil {
			return nil, err
		}
		if config.N == 1 {
			log.Printf("response: %s", rsp)
		}
		return &berf.Result{Status: []string{"200"}}, nil
	case "status":
		fallthrough
	default:
		if config.N == 1 {
			log.Printf("request: %s", &service.StatusRequest{})
		}
		rsp, err := b.client.Status(ctx, &service.StatusRequest{})
		if err != nil {
			return nil, err
		}
		if config.N == 1 {
			log.Printf("response: %s", rsp)
		}
		return &berf.Result{Status: []string{strconv.FormatUint(rsp.Status, 10)}}, nil
	}
}

func (b *bench) Final(ctx context.Context, config *berf.Config) error {
	if *pPool {
		return b.pool.Close()
	}

	return b.conn.Close()
}
