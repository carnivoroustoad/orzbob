package main

import (
	"context"
	"encoding/json"
	"fmt"
	"orzbob/app"
	"orzbob/config"
	"orzbob/daemon"
	"orzbob/log"
	"orzbob/session"
	"orzbob/session/git"
	"orzbob/session/tmux"
	"orzbob/update"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	version     = "dev"
	gitCommit   = "unknown"
	buildDate   = "unknown"
	programFlag string
	autoYesFlag bool
	daemonFlag  bool
	rootCmd     = &cobra.Command{
		Use:   "orz",
		Short: "Orzbob - A terminal-based session manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			log.Initialize(daemonFlag)
			defer log.Close()

			if daemonFlag {
				cfg := config.LoadConfig()
				err := daemon.RunDaemon(cfg)
				log.ErrorLog.Printf("failed to start daemon %v", err)
				return err
			}

			// Check if we're in a git repository
			currentDir, err := filepath.Abs(".")
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			if !git.IsGitRepo(currentDir) {
				return fmt.Errorf("error: orz must be run from within a git repository")
			}

			cfg := config.LoadConfig()

			// Program flag overrides config
			program := cfg.DefaultProgram
			if programFlag != "" {
				program = programFlag
			}
			// AutoYes flag overrides config
			autoYes := cfg.AutoYes
			if autoYesFlag {
				autoYes = true
			}
			if autoYes {
				defer func() {
					if err := daemon.LaunchDaemon(); err != nil {
						log.ErrorLog.Printf("failed to launch daemon: %v", err)
					}
				}()
			}
			// Kill any daemon that's running.
			if err := daemon.StopDaemon(); err != nil {
				log.ErrorLog.Printf("failed to stop daemon: %v", err)
			}

			// Run auto-update check after logging is initialized and before starting the app UI
			// This is done silently and only shows output if an update is available
			if err := runAutoUpdateCheck(); err != nil {
				log.ErrorLog.Printf("auto-update check failed: %v", err)
				// Continue execution even if auto-update fails
			}

			return app.Run(ctx, program, autoYes)
		},
	}

	resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Reset all stored instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Initialize(false)
			defer log.Close()

			state := config.LoadState()
			storage, err := session.NewStorage(state)
			if err != nil {
				return fmt.Errorf("failed to initialize storage: %w", err)
			}
			if err := storage.DeleteAllInstances(); err != nil {
				return fmt.Errorf("failed to reset storage: %w", err)
			}
			fmt.Println("Storage has been reset successfully")

			if err := tmux.CleanupSessions(); err != nil {
				return fmt.Errorf("failed to cleanup tmux sessions: %w", err)
			}
			fmt.Println("Tmux sessions have been cleaned up")

			if err := git.CleanupWorktrees(); err != nil {
				return fmt.Errorf("failed to cleanup worktrees: %w", err)
			}
			fmt.Println("Worktrees have been cleaned up")

			// Kill any daemon that's running.
			if err := daemon.StopDaemon(); err != nil {
				return err
			}
			fmt.Println("daemon has been stopped")

			return nil
		},
	}

	debugCmd = &cobra.Command{
		Use:   "debug",
		Short: "Print debug information like config paths",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadConfig()

			configDir, err := config.GetConfigDir()
			if err != nil {
				return fmt.Errorf("failed to get config directory: %w", err)
			}
			configJson, _ := json.MarshalIndent(cfg, "", "  ")

			fmt.Printf("Config: %s\n%s\n", filepath.Join(configDir, config.ConfigFileName), configJson)

			// Print update information
			fmt.Printf("\nUpdate information:\n")
			fmt.Printf("Current version: v%s\n", version)
			fmt.Printf("Auto-update enabled: %t\n", cfg.EnableAutoUpdate)
			fmt.Printf("Auto-install updates: %t\n", cfg.AutoInstallUpdates)

			if cfg.LastUpdateCheck > 0 {
				lastCheck := time.Unix(cfg.LastUpdateCheck, 0)
				fmt.Printf("Last update check: %s (%s ago)\n",
					lastCheck.Format(time.RFC1123),
					time.Since(lastCheck).Round(time.Second))
			} else {
				fmt.Printf("Last update check: Never\n")
			}

			return nil
		},
	}

	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of orz",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("orz version %s\n", version)
			if gitCommit != "unknown" {
				fmt.Printf("  commit: %s\n", gitCommit)
			}
			if buildDate != "unknown" {
				fmt.Printf("  built: %s\n", buildDate)
			}
			fmt.Printf("  https://github.com/carnivoroustoad/orzbob/releases/tag/v%s\n", version)
		},
	}
)

// runAutoUpdateCheck is a helper function that safely runs the auto-update check
func runAutoUpdateCheck() error {
	// Only run auto-update check for the main command, not for subcommands
	if len(os.Args) > 1 && os.Args[1] == "auto-update" {
		return nil // Skip if we're already running auto-update
	}

	// Run auto-update check
	return update.AutoUpdateCmd.RunE(update.AutoUpdateCmd, []string{})
}

func init() {
	// Set the global version variable for the update package
	update.CurrentVersion = version

	rootCmd.Flags().StringVarP(&programFlag, "program", "p", "",
		"Program to run in new instances (e.g. 'aider --model ollama_chat/gemma3:1b')")
	rootCmd.Flags().BoolVarP(&autoYesFlag, "autoyes", "y", false,
		"[experimental] If enabled, all instances will automatically accept prompts")
	rootCmd.Flags().BoolVar(&daemonFlag, "daemon", false, "Run a program that loads all sessions"+
		" and runs autoyes mode on them.")

	// Hide the daemonFlag as it's only for internal use
	err := rootCmd.Flags().MarkHidden("daemon")
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(debugCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(resetCmd)
	rootCmd.AddCommand(update.UpdateCmd)
	rootCmd.AddCommand(update.AutoUpdateCmd)
	rootCmd.AddCommand(cloudCmd)
	rootCmd.AddCommand(loginCmd)
}

func main() {
	// Initialize log for error handling
	log.Initialize(false)
	defer log.Close()

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}
