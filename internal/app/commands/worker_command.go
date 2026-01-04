package commands

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// GetWorkerCommand возвращает команду для запуска воркеров
func GetWorkerCommand() *cli.Command {
	return &cli.Command{
		Name:  "worker",
		Usage: "Start background workers",
		Description: `Start background workers for processing queues, 
sending notifications, etc.`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "queue",
				Aliases: []string{"q"},
				Value:   "default",
				Usage:   "Queue name to process",
			},
			&cli.IntFlag{
				Name:  "workers",
				Value: 5,
				Usage: "Number of worker goroutines",
			},
			&cli.StringFlag{
				Name:  "redis",
				Value: "redis://localhost:6379",
				Usage: "Redis connection URL",
			},
		},
		Action: func(c *cli.Context) error {
			ctx, err := NewCommandContext(c)
			if err != nil {
				return err
			}
			defer ctx.Logger.Sync()

			ctx.Logger.Info("Starting background workers",
				zap.String("queue", c.String("queue")),
				zap.Int("workers", c.Int("workers")))

			// Запускаем воркеры
			return startWorkers(ctx, c)
		},
	}
}

func startWorkers(ctx *CommandContext, c *cli.Context) error {
	queueName := c.String("queue")
	workerCount := c.Int("workers")

	fmt.Printf("🚀 Starting %d workers for queue '%s'\n", workerCount, queueName)
	fmt.Println("Press Ctrl+C to stop")

	// Симуляция работы воркеров
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)

	// Горутины воркеров
	for i := 1; i <= workerCount; i++ {
		go func(workerID int) {
			for {
				select {
				case <-ticker.C:
					ctx.Logger.Debug("Worker processing job",
						zap.Int("worker_id", workerID),
						zap.String("queue", queueName))
					fmt.Printf("Worker %d processed job from %s\n", workerID, queueName)
				case <-done:
					return
				}
			}
		}(i)
	}

	// Ожидаем сигнал завершения
	<-make(chan struct{})
	return nil
}
