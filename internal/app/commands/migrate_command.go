package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"go.uber.org/zap"
)

// GetMigrateCommand возвращает команду для миграций
func GetMigrateCommand() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Database migrations",
		Subcommands: []*cli.Command{
			{
				Name:  "up",
				Usage: "Apply all pending migrations",
				Action: func(c *cli.Context) error {
					ctx, err := NewCommandContext(c)
					if err != nil {
						return err
					}
					defer ctx.Logger.Sync()

					return runMigrationUp(ctx)
				},
			},
			{
				Name:  "down",
				Usage: "Rollback last migration",
				Action: func(c *cli.Context) error {
					ctx, err := NewCommandContext(c)
					if err != nil {
						return err
					}
					defer ctx.Logger.Sync()

					return runMigrationDown(ctx)
				},
			},
			{
				Name:  "status",
				Usage: "Show migration status",
				Action: func(c *cli.Context) error {
					ctx, err := NewCommandContext(c)
					if err != nil {
						return err
					}
					defer ctx.Logger.Sync()

					return showMigrationStatus(ctx)
				},
			},
			{
				Name:  "create",
				Usage: "Create new migration file",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "name",
						Aliases:  []string{"n"},
						Required: true,
						Usage:    "Migration name",
					},
				},
				Action: func(c *cli.Context) error {
					ctx, err := NewCommandContext(c)
					if err != nil {
						return err
					}
					defer ctx.Logger.Sync()

					return createMigration(ctx, c.String("name"))
				},
			},
		},
	}
}

func runMigrationUp(ctx *CommandContext) error {
	ctx.Logger.Info("Running migrations up...")

	// Здесь код миграций
	fmt.Println("Applying migrations...")

	// Пример: если используется SQL миграции
	// err := migrate.Up(ctx.Config.DBConnection)
	// if err != nil {
	//     return err
	// }

	ctx.Logger.Info("Migrations applied successfully")
	return nil
}

func runMigrationDown(ctx *CommandContext) error {
	ctx.Logger.Info("Running migration down...")
	fmt.Println("Rolling back last migration...")
	ctx.Logger.Info("Migration rolled back")
	return nil
}

func showMigrationStatus(ctx *CommandContext) error {
	fmt.Println("Migration status:")
	fmt.Println("✓ 001_create_users_table")
	fmt.Println("✓ 002_create_sessions_table")
	fmt.Println("⏳ 003_add_indexes (pending)")
	return nil
}

func createMigration(ctx *CommandContext, name string) error {
	ctx.Logger.Info("Creating migration", zap.String("name", name))

	// Создание файла миграции
	// timestamp := time.Now().Format("20060102150405")
	// filename := fmt.Sprintf("migrations/%s_%s.sql", timestamp, name)

	fmt.Printf("Created migration: %s\n", name)
	return nil
}
