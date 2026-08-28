package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lcorneliussen/md365/internal/cal"
	"github.com/lcorneliussen/md365/internal/output"
	"github.com/spf13/cobra"
)

var (
	calAccount   string
	calFrom      string
	calTo        string
	calSearch    string
	calSubject   string
	calStart     string
	calEnd       string
	calLocation  string
	calBody      string
	calID        string
	calFile      string
	calAttendees []string
	calForce     bool
	calNoCache   bool
)

// calCmd represents the cal command
var calCmd = &cobra.Command{
	Use:   "cal",
	Short: "Calendar commands",
	Long:  `Manage calendar events.`,
}

// calListCmd represents the cal list command
var calListCmd = &cobra.Command{
	Use:   "list",
	Short: "List calendar events",
	Long:  `List calendar events from local Markdown files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse dates
		var fromDate, toDate time.Time
		var err error

		if calFrom != "" {
			fromDate, err = time.Parse("2006-01-02", calFrom)
			if err != nil {
				return usageError(err.Error())
			}
		} else {
			fromDate = time.Now()
		}

		if calTo != "" {
			toDate, err = time.Parse("2006-01-02", calTo)
			if err != nil {
				return usageError(err.Error())
			}
			// Set to end of day
			toDate = toDate.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		} else {
			toDate = time.Now().AddDate(0, 0, 14).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}

		events, err := cal.List(cfg, fromDate, toDate, calSearch, calAccount, calNoCache)
		if err != nil {
			return err
		}

		if writer.IsHuman() {
			printCalendarEvents(cmd, events)
			return nil
		}
		return writeOK(events,
			output.WithSummary(fmt.Sprintf("%d calendar events", len(events))),
			output.WithMeta("source", sourceName(calNoCache)),
		)
	},
}

// calCreateCmd represents the cal create command
var calCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create calendar event",
	Long:  `Create a new calendar event via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		if calAccount == "" || calSubject == "" || calStart == "" || calEnd == "" {
			cmd.Help()
			os.Exit(1)
			return
		}

		filePath, err := cal.Create(cfg, calAccount, calSubject, calStart, calEnd, calLocation, calBody, calAttendees, calForce)
		if err != nil {
			fatal(err)
		}
		if writer.IsHuman() {
			fmt.Fprintf(cmd.OutOrStdout(), "Event created: %s\n", filePath)
			return
		}
		_ = writeOK(map[string]string{
			"account":   calAccount,
			"file_path": filePath,
		}, output.WithSummary("Event created"))
	},
}

// calDeleteCmd represents the cal delete command
var calDeleteCmd = &cobra.Command{
	Use:   "delete [file]",
	Short: "Delete calendar event",
	Long:  `Delete a calendar event via Microsoft Graph API.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Check if file path is provided as argument
		if len(args) > 0 {
			calFile = args[0]
		}

		filePath, err := cal.Delete(cfg, calAccount, calID, calFile)
		if err != nil {
			fatal(err)
		}
		if writer.IsHuman() {
			if filePath != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Event deleted: %s\n", filePath)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Event deleted (local file not found)")
			}
			return
		}
		_ = writeOK(map[string]string{
			"account":   calAccount,
			"id":        calID,
			"file_path": filePath,
		}, output.WithSummary("Event deleted"))
	},
}

func init() {
	// cal list
	calListCmd.Flags().StringVar(&calFrom, "from", "", "Start date (YYYY-MM-DD)")
	calListCmd.Flags().StringVar(&calTo, "to", "", "End date (YYYY-MM-DD)")
	calListCmd.Flags().StringVar(&calSearch, "search", "", "Search query")
	calListCmd.Flags().StringVar(&calAccount, "account", "", "Filter by account")
	calListCmd.Flags().BoolVar(&calNoCache, "no-cache", false, "Read directly from Microsoft Graph instead of local Markdown")

	// cal create
	calCreateCmd.Flags().StringVar(&calAccount, "account", "", "Account (required)")
	calCreateCmd.Flags().StringVar(&calSubject, "subject", "", "Event subject (required)")
	calCreateCmd.Flags().StringVar(&calStart, "start", "", "Start date/time (required)")
	calCreateCmd.Flags().StringVar(&calEnd, "end", "", "End date/time (required)")
	calCreateCmd.Flags().StringVar(&calLocation, "location", "", "Location")
	calCreateCmd.Flags().StringVar(&calBody, "body", "", "Body text")
	calCreateCmd.Flags().StringSliceVar(&calAttendees, "attendees", []string{}, "Attendee emails (comma-separated)")
	calCreateCmd.Flags().BoolVar(&calForce, "force", false, "Bypass cross-tenant checks")

	// cal delete
	calDeleteCmd.Flags().StringVar(&calAccount, "account", "", "Account")
	calDeleteCmd.Flags().StringVar(&calID, "id", "", "Event ID")

	calCmd.AddCommand(calListCmd)
	calCmd.AddCommand(calCreateCmd)
	calCmd.AddCommand(calDeleteCmd)
}

func printCalendarEvents(cmd *cobra.Command, events []cal.EventInfo) {
	for _, event := range events {
		startDate := event.Start.Format("2006-01-02 Mon")
		startTime := event.Start.Format("15:04")
		endTime := event.End.Format("15:04")

		line := fmt.Sprintf("%s %s-%s %-30s [%s]",
			startDate, startTime, endTime, truncateHuman(event.Subject, 30), event.Account)

		if event.Location != "" {
			line += fmt.Sprintf(" 📍 %s", event.Location)
		}

		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
}

func sourceName(noCache bool) string {
	if noCache {
		return "graph"
	}
	return "cache"
}

func truncateHuman(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return strings.TrimSpace(s[:maxLen])
}
