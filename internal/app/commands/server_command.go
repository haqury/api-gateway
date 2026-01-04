package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"

	"api-gateway/internal/app"
)

// GetServerCommand возвращает команду для запуска сервера
func GetServerCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "Start API Gateway server",
		Description: `Start the API Gateway server with HTTP/HTTPS support.
		
Examples:
  api-gateway server --port 8080
  api-gateway server --port 443 --tls --cert server.crt --key server.key`,
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "port",
				Aliases: []string{"p"},
				Value:   8080,
				Usage:   "Server port",
			},
			&cli.StringFlag{
				Name:  "host",
				Value: "0.0.0.0",
				Usage: "Server host",
			},
			&cli.BoolFlag{
				Name:  "tls",
				Value: false,
				Usage: "Enable TLS",
			},
			&cli.StringFlag{
				Name:  "cert",
				Usage: "TLS certificate file",
			},
			&cli.StringFlag{
				Name:  "key",
				Usage: "TLS key file",
			},
			&cli.BoolFlag{
				Name:    "debug",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "Enable debug mode",
			},
		},
		Action: func(c *cli.Context) error {
			ctx, err := NewCommandContext(c)
			if err != nil {
				return err
			}
			defer ctx.Logger.Sync()

			ctx.Logger.Info("Starting API Gateway server",
				zap.Int("port", c.Int("port")),
				zap.String("host", c.String("host")),
				zap.Bool("debug", c.Bool("debug")),
				zap.Bool("tls", c.Bool("tls")))

			// Создаем приложение
			application := app.NewApplicationWithConfig(ctx.Config, ctx.Logger)

			// Запускаем сервер
			if c.Bool("tls") {
				cert := c.String("cert")
				key := c.String("key")
				if cert == "" || key == "" {
					return fmt.Errorf("both --cert and --key are required for TLS")
				}
				return application.RunTLS(cert, key)
			}

			return application.Run()
		},
	}
}
