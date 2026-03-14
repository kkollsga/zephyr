package command

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kristianweb/zephyr/internal/fuzzy"
)

// Handler is a function that executes a command.
type Handler func() error

// Command represents a registered command.
type Command struct {
	ID         string
	Title      string
	Handler    Handler
	Keybinding string // display string like "Cmd+S"
}

// Registry holds all registered commands.
type Registry struct {
	commands map[string]*Command
	order    []string
}

// NewRegistry creates an empty command registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]*Command),
	}
}

// Register adds a command. Returns error if ID already exists.
func (r *Registry) Register(cmd *Command) error {
	if _, exists := r.commands[cmd.ID]; exists {
		return fmt.Errorf("command %q already registered", cmd.ID)
	}
	r.commands[cmd.ID] = cmd
	r.order = append(r.order, cmd.ID)
	return nil
}

// Execute runs the command with the given ID.
func (r *Registry) Execute(id string) error {
	cmd, ok := r.commands[id]
	if !ok {
		return fmt.Errorf("command %q not found", id)
	}
	if cmd.Handler == nil {
		return nil
	}
	return cmd.Handler()
}

// Get returns a command by ID, or nil.
func (r *Registry) Get(id string) *Command {
	return r.commands[id]
}

// All returns all commands in registration order.
func (r *Registry) All() []*Command {
	result := make([]*Command, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.commands[id])
	}
	return result
}

// Search returns commands matching the query, sorted by relevance.
func (r *Registry) Search(query string) []*Command {
	if query == "" {
		return r.All()
	}

	titles := make([]string, 0, len(r.order))
	titleToCmd := make(map[string]*Command)
	for _, id := range r.order {
		cmd := r.commands[id]
		titles = append(titles, cmd.Title)
		titleToCmd[cmd.Title] = cmd
	}

	matches := fuzzy.RankMatches(query, titles)
	result := make([]*Command, 0, len(matches))
	for _, m := range matches {
		result = append(result, titleToCmd[m.Text])
	}
	return result
}

// SearchByID returns commands whose IDs contain the query.
func (r *Registry) SearchByID(query string) []*Command {
	query = strings.ToLower(query)
	var result []*Command
	for _, id := range r.order {
		if strings.Contains(strings.ToLower(id), query) {
			result = append(result, r.commands[id])
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Count returns the number of registered commands.
func (r *Registry) Count() int {
	return len(r.commands)
}
