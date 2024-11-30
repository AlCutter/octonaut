package cmd

import (
	"context"
	"database/sql"
	"strings"

	"github.com/AlCutter/octonaut/internal/octonaut"
	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "octonaut",
	Short: "A tool for interacting with your Octopus Energy account",
}

var (
	EndPoint string
	Account  string
	Key      string
	DBPath   string
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		log.Fatal("Execute: %v", err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&EndPoint, "endpoint", "https://api.octopus.energy/", "Base URL of the Octopus API.")
	rootCmd.PersistentFlags().StringVar(&Account, "account", "", "Octopus Account e.g. A-123456.")
	rootCmd.PersistentFlags().StringVar(&Key, "key", "", "Octopus API key.")
	rootCmd.PersistentFlags().StringVar(&DBPath, "db", "./octonaut.sqlite3", "SQLite3 DB path and filename.")
}

func MustNewFromFlags(ctx context.Context) (*octonaut.Octonaut, func() error) {
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		log.Fatalf("Failed to open DB (%q): %v", DBPath, err)
	}

	u := EndPoint
	if !strings.HasSuffix(u, "/") {
		u += "/"
	}

	r, err := octonaut.New(ctx, Account, Key, u, db)
	if err != nil {
		log.Fatalf("New: %v", err)
	}

	return r, db.Close
}
