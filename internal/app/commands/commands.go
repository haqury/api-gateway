package commands

import (
	"github.com/urfave/cli/v2"
)

// GetCommands возвращает все доступные команды
func GetCommands() []*cli.Command {
	return []*cli.Command{
		GetServerCommand(),
		GetMigrateCommand(),
		GetWorkerCommand(),
		GetVersionCommand(),
		GetTestCommand(),
		GetGenerateCommand(),
	}
}
