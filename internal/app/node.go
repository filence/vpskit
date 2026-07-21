package app

import (
	"errors"
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"vpskit.local/vpskit/internal/model"
	"vpskit.local/vpskit/internal/platform"
)

var nodeIDPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)

func nodeMetadataFromOptions(options InstallOptions) model.NodeMetadata {
	priority := options.NodePriority
	if priority == 0 {
		priority = 100
	}
	return model.NodeMetadata{
		ID:                    defaultString(options.NodeID, "node-main"),
		DisplayName:           defaultString(options.NodeName, "VPSKit"),
		Provider:              strings.TrimSpace(options.Provider),
		Country:               strings.ToUpper(strings.TrimSpace(options.Country)),
		City:                  strings.TrimSpace(options.City),
		Priority:              priority,
		EnabledInSubscription: true,
	}
}

func validateNodeMetadata(node model.NodeMetadata) error {
	if !nodeIDPattern.MatchString(strings.TrimSpace(node.ID)) {
		return fmt.Errorf("invalid node ID %q; use 1-63 lowercase letters, digits, or hyphens", node.ID)
	}
	name := strings.TrimSpace(node.DisplayName)
	if name == "" || len([]rune(name)) > 64 {
		return errors.New("node display name must contain 1-64 characters")
	}
	for _, value := range []string{name, node.Provider, node.City} {
		for _, character := range value {
			if unicode.IsControl(character) {
				return errors.New("node metadata must not contain control characters")
			}
		}
	}
	if len([]rune(node.Provider)) > 64 || len([]rune(node.City)) > 64 {
		return errors.New("node provider and city labels must not exceed 64 characters")
	}
	if node.Country != "" && !countryCodePattern.MatchString(node.Country) {
		return errors.New("node country must be an uppercase two-letter ISO code")
	}
	if node.Priority < 1 || node.Priority > 1000 {
		return errors.New("node priority must be between 1 and 1000")
	}
	return nil
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func runNode(arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: vpskit node <show|modify> [options]")
	}
	if !platform.IsRoot() {
		return errors.New("node inspection and mutation require root privileges")
	}
	state, err := readInstalledState()
	if err != nil {
		return err
	}
	if arguments[0] == "show" {
		if len(arguments) != 1 {
			return errors.New("usage: vpskit node show")
		}
		return printJSON(commandResult{Command: "node show", Status: "PASS", Detail: state.Node})
	}
	if arguments[0] != "modify" {
		return fmt.Errorf("unsupported node operation %q", arguments[0])
	}
	flags := flag.NewFlagSet("node modify", flag.ContinueOnError)
	displayName := flags.String("display-name", "", "client display name prefix")
	provider := flags.String("provider", "", "VPS provider label")
	country := flags.String("country", "", "ISO country code")
	city := flags.String("city", "", "city label")
	priority := flags.Int("priority", 0, "subscription node priority")
	tags := flags.String("tags", "", "comma-separated node tags")
	subscriptionEnabled := flags.String("subscription-enabled", "", "true or false")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	visited := map[string]bool{}
	flags.Visit(func(item *flag.Flag) { visited[item.Name] = true })
	if len(visited) == 0 {
		return errors.New("node modify requires at least one metadata option")
	}
	updated := state
	if visited["display-name"] {
		updated.Node.DisplayName = strings.TrimSpace(*displayName)
	}
	if visited["provider"] {
		updated.Node.Provider = strings.TrimSpace(*provider)
	}
	if visited["country"] {
		updated.Node.Country = strings.ToUpper(strings.TrimSpace(*country))
	}
	if visited["city"] {
		updated.Node.City = strings.TrimSpace(*city)
	}
	if visited["priority"] {
		updated.Node.Priority = *priority
	}
	if visited["tags"] {
		updated.Node.Tags = normalizedTags(*tags)
	}
	if visited["subscription-enabled"] {
		value, err := strconv.ParseBool(strings.TrimSpace(*subscriptionEnabled))
		if err != nil {
			return errors.New("--subscription-enabled must be true or false")
		}
		updated.Node.EnabledInSubscription = value
	}
	if err := validateNodeMetadata(updated.Node); err != nil {
		return err
	}
	changes := clientFacingChanges(state, updated)
	if len(changes) == 0 {
		return errors.New("node metadata is unchanged")
	}
	updated.ConfigRevision = state.ConfigRevision + 1
	secrets, err := readInstalledSecrets()
	if err != nil {
		return err
	}
	commit, err := commitManagedStateChange(state, updated, secrets, "node modify", "node", changes)
	if err != nil {
		return err
	}
	return printJSON(commandResult{Command: "node modify", Status: "PASS", Detail: map[string]any{
		"node":                   commit.State.Node,
		"config_revision":        commit.State.ConfigRevision,
		"changed_client_fields":  changes,
		"client_update_required": true,
		"transaction_id":         commit.TransactionID,
		"previous_backup_id":     commit.BackupID,
		"subscription_publish":   commit.SubscriptionPublish,
	}})
}

func normalizedTags(value string) []string {
	seen := map[string]struct{}{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			seen[item] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for item := range seen {
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}
