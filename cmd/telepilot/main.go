package main

import (
	"context"
	"fmt"
	"log" // TODO: Consider using slog.
	"os"
	"path"

	"github.com/urfave/cli/v3"
	"google.golang.org/grpc"

	pb "go.creack.net/telepilot/api/v1"
	"go.creack.net/telepilot/pkg/apiclient"
	"go.creack.net/telepilot/pkg/tlsconfig"
)

//nolint:funlen // Acceptable for CLI definition.
func main() {
	var client pb.TelePilotServiceClient
	var jobID string

	var conn *grpc.ClientConn

	jobIDArg := &cli.StringArg{
		Name:      "<job_id>",
		UsageText: "<job_id>",
		Min:       1,
		Max:       1,
	}
	parseJobID := func(_ context.Context, cmd *cli.Command) error {
		jobID = cmd.Args().First()
		return nil
	}

	cmd := &cli.Command{
		// Before any command, load the certs and connect to the server.
		Before: func(_ context.Context, cmd *cli.Command) error {
			certDir := cmd.String("certs")
			user := cmd.String("user")
			tlsConfig, err := tlsconfig.LoadTLSConfig(
				path.Join(certDir, "client-"+user+".pem"),
				path.Join(certDir, "client-"+user+"-key.pem"),
				path.Join(certDir, "ca.pem"),
				true,
			)
			if err != nil {
				return fmt.Errorf("load tls config for %q from %q: %w", user, certDir, err)
			}
			// TODO: Consider making the addr a flag.
			c, err := apiclient.NewClient(tlsConfig, "localhost:9090")
			if err != nil {
				return fmt.Errorf("connect: %w", err)
			}
			conn = c
			client = pb.NewTelePilotServiceClient(conn)
			return nil
		},
		// After any command, disconnect.
		After: func(_ context.Context, _ *cli.Command) error {
			if conn != nil {
				return conn.Close()
			}
			return nil
		},
		Commands: []*cli.Command{
			{
				Name:      "start",
				Usage:     "Start a new Job.",
				UsageText: "telepilot [global options] start <command> [arguments...]",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					if !cmd.Args().Present() {
						return cli.ShowSubcommandHelp(cmd)
					}
					jobID, err := client.StartJob(ctx, &pb.StartJobRequest{Command: cmd.Args().First(), Args: cmd.Args().Tail()})
					if err != nil {
						return err //nolint:wrapcheck // No wrap needed here.
					}
					fmt.Fprintln(cmd.Writer, jobID)
					return nil
				},
			},
			{
				Name:  "stop",
				Usage: "Stops a running Job. Sends SIGKILL to ensure termination.",
				Action: func(ctx context.Context, _ *cli.Command) error {
					_, err := client.StopJob(ctx, &pb.StopJobRequest{JobId: jobID})
					return err
				},
				Arguments: []cli.Argument{jobIDArg},
				Before:    parseJobID,
			},
			{
				Name:  "status",
				Usage: "Lookup the status of a Job.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					status, err := apiclient.GetJobStatus(ctx, client, jobID)
					if err != nil {
						return err //nolint:wrapcheck // No wrap needed here.
					}
					fmt.Fprintln(cmd.Writer, status)
					return nil
				},
				Arguments: []cli.Argument{jobIDArg},
				Before:    parseJobID,
			},
			{
				Name:  "logs",
				Usage: "Streams logs from a Job until it exits.",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					return apiclient.StreamLogs(ctx, client, jobID, cmd.Writer)
				},
				Arguments: []cli.Argument{jobIDArg},
				Before:    parseJobID,
			},
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "certs",
				Value: "./certs",
				Usage: "Certs directory. " +
					"Expecting <certdir>/ca.pem, <certdir>/client-<user>.pem and <certdir>/client-<user>-key.pem.",
			},
			&cli.StringFlag{
				Name:  "user",
				Value: "alice",
				Usage: "Client user name. Cert and key expected in <certdir>.",
			},
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
