// TUI for manipulating Metrics
// Using Bubbletea:
// Create a list from the database
// Add to database
// Update value
// Increment value/Decrement Value
//

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	pb "github.com/qjs/quanti-tea/server/proto" // Adjust the import path as necessary
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// =============================================================
// Constants and Styles
// =============================================================

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#32a491")).
			Padding(0, 1)

	selectedTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#78a547")).
				Padding(0, 1)

	selectedDescStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#c8d6bf")).
				Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8bc8ee")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000")).
				Render
)

// =============================================================
// Data Structures
// =============================================================

// Metric represents a single metric.
type Metric struct {
	MetricName string
	Type       string
	Unit       string
	Value      float64
	ResetDaily bool
}

// Implement the list.Item interface for Metric
func (m Metric) Title() string { return m.MetricName }
func (m Metric) Description() string {
	return fmt.Sprintf("Type: %s | Unit: %s | Value: %.2f, Reset Daily: %t", m.Type, m.Unit, m.Value, m.ResetDaily)
}
func (m Metric) FilterValue() string { return m.MetricName }

// =============================================================
// Key Bindings
// =============================================================

// Define key bindings
type keyMap struct {
	Quit key.Binding
	Add  key.Binding
	Inc  key.Binding
	Dec  key.Binding
	Upd  key.Binding
	Ref  key.Binding
	Del  key.Binding
}

func newKeyMap() *keyMap {
	return &keyMap{
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Add: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "add metric"),
		),
		Inc: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "increment metric"),
		),
		Dec: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "decrement metric"),
		),
		Upd: key.NewBinding(
			key.WithKeys("u"),
			key.WithHelp("u", "update metric"),
		),
		Ref: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "refresh metrics"),
		),
		Del: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "delete metrics"),
		),
	}
}

// =============================================================
// Model
// =============================================================

// model defines the state of the TUI application.
type model struct {
	metrics      []Metric
	list         list.Model              // The list component
	input        textinput.Model         // Text input for user input
	status       string                  // Status message
	client       pb.MetricsServiceClient // gRPC client
	keys         *keyMap                 // Key bindings
	quitting     bool                    // Quit flag
	action       string                  // Current action: add, inc, dec, upd
	selected     int                     // Selected metric index
	lastUpdated  time.Time               // Last update timestamp
	delegateKeys *delegateKeyMap
}

// =============================================================
// Initialization
// =============================================================

// initialModel initializes the TUI model.
func initialModel(client pb.MetricsServiceClient) model {
	// Initialize key bindings
	keys := newKeyMap()
	delegateKeys := newDelegateKeyMap()
	delegate := newItemDelegate(delegateKeys)

	// Initialize the list
	items := []list.Item{} // Start with an empty list
	metricList := list.New(items, delegate, 0, 0)
	metricList.Title = ""
	metricList.Styles.Title = titleStyle
	metricList.Help.Styles.FullKey = helpStyle
	metricList.Help.Styles.ShortKey = helpStyle

	metricList.SetShowHelp(true)
	metricList.SetFilteringEnabled(false)
	metricList.SetShowStatusBar(true)
	metricList.SetShowPagination(true)
	metricList.SetShowTitle(true)
	metricList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Quit,
			keys.Add,
			keys.Dec,
			keys.Inc,
			keys.Upd,
			keys.Ref,
			keys.Del,
		}
	}

	// Initialize text input
	ti := textinput.New()
	ti.Placeholder = ""
	ti.Focus()
	ti.CharLimit = 32
	ti.Width = 20

	return model{
		list:         metricList,
		input:        ti,
		status:       "Welcome! Press 'a' to add a metric.",
		client:       client,
		keys:         keys,
		quitting:     false,
		action:       "",
		selected:     0,
		delegateKeys: delegateKeys,
	}
}

func (m model) Init() tea.Cmd {
	return m.fetchMetrics()
}

// =============================================================
// Messages
// =============================================================

// metricsMsg is a custom message containing the metrics data.
type metricsMsg struct {
	metrics    []Metric
	lastUpdate time.Time
}

// errMsg is a custom message containing an error.
type errMsg struct {
	err error
}

// Constants for layout calculation
const (
	headerHeight     = 3                                         // Header lines
	statusHeight     = 2                                         // Status message and spacing
	inputHeight      = 3                                         // Action + input + spacing
	paddingTopBottom = 2                                         // appStyle.Padding(1, 2) adds 1 top and 1 bottom
	totalExtra       = headerHeight + statusHeight + inputHeight // 8
	minListHeight    = 5
)

// Update handles incoming messages and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Calculate available height for the list
		availableHeight := msg.Height - paddingTopBottom - totalExtra
		if availableHeight < minListHeight {
			availableHeight = minListHeight
		}

		// Calculate available width for the list
		listWidth := msg.Width - 4 // 2 left and 2 right padding
		if listWidth < 20 {
			listWidth = 20 // minimum width
		}

		m.list.SetSize(listWidth, availableHeight)
		return m, nil

	case metricsMsg:
		// Update metrics list
		m.metrics = msg.metrics
		m.list.SetItems(toListItems(m.metrics))
		m.lastUpdated = msg.lastUpdate
		m.status = fmt.Sprintf("Metrics updated at %s", m.lastUpdated.Format(time.RFC1123))
		return m, nil

	case errMsg:
		// Display error message
		m.status = fmt.Sprintf("Error: %v", msg.err)
		return m, nil

	case actionCompletedMsg:
		// Update status based on the completed action
		m.status = fmt.Sprintf("Action '%s' completed successfully.", msg.action)
		// Trigger fetchMetrics to refresh the list
		return m, m.fetchMetrics()

	case tea.KeyMsg:
		if m.action == "" {
			switch {
			case key.Matches(msg, m.keys.Quit):
				m.quitting = true
				return m, tea.Quit

			case key.Matches(msg, m.keys.Add):
				if m.action != "" {
					m.status = "Finish the current action first."
					return m, nil
				}
				m.action = "add"
				m.input.Placeholder = "Name,Type,Unit,(Y/N) reset daily"
				m.input.SetValue("")
				m.input.Focus()
				m.status = "Enter Metric Name and Type (comma separated):"
				return m, nil

			case key.Matches(msg, m.keys.Del):
				if len(m.metrics) == 0 {
					m.status = "No metrics available to delete."
					return m, nil
				}
				m.action = "confirm_del" // Set action to confirmation state
				selectedMetric := m.metrics[m.list.Index()]
				m.input.Placeholder = fmt.Sprintf("Delete %s (Y/N)?", selectedMetric.MetricName)
				m.input.SetValue("")
				m.input.Focus()
				m.status = fmt.Sprintf("Are you sure you want to delete '%s'? (Y/N)", selectedMetric.MetricName)
				return m, nil

			case key.Matches(msg, m.keys.Inc):
				if len(m.metrics) == 0 {
					m.status = "No metrics available to increment."
					return m, nil
				}
				m.action = "inc"
				selectedMetric := m.metrics[m.list.Index()]
				m.input.Placeholder = fmt.Sprintf("Increment '%s' by", selectedMetric.MetricName)
				m.input.SetValue("")
				m.input.Focus()
				m.status = fmt.Sprintf("Enter increment value for '%s':", selectedMetric.MetricName)
				return m, nil

			case key.Matches(msg, m.keys.Dec):
				if len(m.metrics) == 0 {
					m.status = "No metrics available to decrement."
					return m, nil
				}
				m.action = "dec"
				selectedMetric := m.metrics[m.list.Index()]
				m.input.Placeholder = fmt.Sprintf("Decrement '%s' by", selectedMetric.MetricName)
				m.input.SetValue("")
				m.input.Focus()
				m.status = fmt.Sprintf("Enter decrement value for '%s':", selectedMetric.MetricName)
				return m, nil

			case key.Matches(msg, m.keys.Upd):
				if len(m.metrics) == 0 {
					m.status = "No metrics available to update."
					return m, nil
				}
				m.action = "upd"
				selectedMetric := m.metrics[m.list.Index()]
				m.input.Placeholder = fmt.Sprintf("Update '%s' to", selectedMetric.MetricName)
				m.input.SetValue("")
				m.input.Focus()
				m.status = fmt.Sprintf("Enter new value for '%s':", selectedMetric.MetricName)
				return m, nil

			case key.Matches(msg, m.keys.Ref):
				m.status = "Refreshing metrics..."
				return m, m.fetchMetrics()
			}
		}
	}

	// If an action is active, handle text input
	if m.action != "" {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.Type {
			case tea.KeyEnter:
				input := strings.TrimSpace(m.input.Value())
				if input == "" {
					m.status = "Input cannot be empty."
					m.action = ""
					m.input.Blur()
					return m, nil
				}

				var cmd tea.Cmd

				switch m.action {
				case "add":
					parts := strings.Split(input, ",")
					if len(parts) < 3 || len(parts) > 4 {
						m.status = "Invalid format. Use 'Name,Type,Unit[,ResetDaily (Y/N)]'."
						m.action = ""
						m.input.Blur()
						return m, nil
					}
					name := strings.TrimSpace(parts[0])
					typ := strings.TrimSpace(parts[1])
					unit := strings.TrimSpace(parts[2])

					if name == "" || typ == "" {
						m.status = "Name and Type cannot be empty."
						m.action = ""
						m.input.Blur()
						return m, nil
					}

					// Default the resetDaily flag to false
					resetDaily := false

					// Check if the optional ResetDaily flag (4th parameter) is provided
					if len(parts) == 4 {
						resetInput := strings.TrimSpace(parts[3])
						if resetInput == "Y" || resetInput == "y" {
							resetDaily = true
						} else if resetInput != "N" && resetInput != "n" {
							m.status = "Invalid ResetDaily flag. Use 'Y' or 'N'."
							m.action = ""
							m.input.Blur()
							return m, nil
						}
					}

					// Call the modified addMetric with the resetDaily flag
					cmd = m.addMetric(name, typ, unit, resetDaily)

				case "confirm_del":
					val := strings.TrimSpace(strings.ToLower(input))
					selectedMetric := m.metrics[m.list.Index()]
					if val == "y" || val == "yes" {
						// User confirmed deletion
						cmd := m.delMetric(selectedMetric.MetricName)
						m.action = ""
						m.input.Blur()
						m.status = fmt.Sprintf("Deleting metric '%s'...", selectedMetric.MetricName)
						return m, cmd
					} else if val == "n" || val == "no" {
						// User canceled deletion
						m.action = ""
						m.input.Blur()
						m.status = fmt.Sprintf("Deletion of metric '%s' canceled.", selectedMetric.MetricName)
						return m, nil
					} else {
						// Invalid input, prompt again
						m.status = "Please enter 'Y' or 'N'."
						return m, nil
					}
				case "inc":
					value, err := strconv.ParseFloat(input, 64)
					if err != nil {
						m.status = "Invalid increment value."
						m.action = ""
						m.input.Blur()
						return m, nil
					}
					selectedMetric := m.metrics[m.list.Index()]
					cmd = m.incrementMetric(selectedMetric.MetricName, value)

				case "dec":
					value, err := strconv.ParseFloat(input, 64)
					if err != nil {
						m.status = "Invalid decrement value."
						m.action = ""
						m.input.Blur()
						return m, nil
					}
					selectedMetric := m.metrics[m.list.Index()]
					cmd = m.decrementMetric(selectedMetric.MetricName, value)

				case "upd":
					value, err := strconv.ParseFloat(input, 64)
					if err != nil {
						m.status = "Invalid update value."
						m.action = ""
						m.input.Blur()
						return m, nil
					}
					selectedMetric := m.metrics[m.list.Index()]
					cmd = m.updateMetric(selectedMetric.MetricName, value)
				}

				m.action = ""
				m.input.Blur()
				m.status = "Processing..."
				return m, cmd

			case tea.KeyEsc:
				m.action = ""
				m.status = "Action cancelled."
				m.input.Blur()
				return m, nil
			}
		}

		// Update the text input
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		// Update the list component
		newList, cmd := m.list.Update(msg)
		m.list = newList
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// =============================================================
// View
// =============================================================

// View renders the UI.
func (m model) View() string {
	var sb strings.Builder

	// Header
	sb.WriteString(titleStyle.Render("Quanti-Tea Metrics\n"))
	//sb.WriteString(titleStyle.Render("======================\n"))

	// Metrics List
	if len(m.metrics) == 0 {
		sb.WriteString("No metrics available.\n")
	} else {
		sb.WriteString(m.list.View())
	}

	sb.WriteString("\n")

	// Status Message
	if strings.HasPrefix(m.status, "Error:") {
		sb.WriteString(fmt.Sprintf("Status: %s\n\n", errorMessageStyle(m.status)))
	} else {
		sb.WriteString(fmt.Sprintf("Status: %s\n\n", statusMessageStyle(m.status)))
	}

	// Input Form
	if m.action != "" {
		sb.WriteString(fmt.Sprintf("Action: %s\n", strings.ToUpper(m.action)))
		sb.WriteString(m.input.View())
		sb.WriteString("\n")
	}

	return appStyle.Render(sb.String())
}

// =============================================================
// Helper Functions
// =============================================================

// toListItems converts a slice of Metric to a slice of list.Item.
func toListItems(metrics []Metric) []list.Item {
	items := make([]list.Item, len(metrics))
	for i, m := range metrics {
		items[i] = m
	}
	return items
}

// =============================================================
// RPC Commands
// =============================================================

// fetchMetrics retrieves the list of metrics from the server.
func (m model) fetchMetrics() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := m.client.GetMetrics(ctx, &pb.GetMetricsRequest{})
		if err != nil {
			return errMsg{err}
		}

		metrics := []Metric{}
		for _, metric := range resp.Metrics {
			metrics = append(metrics, Metric{
				MetricName: metric.MetricName,
				Type:       metric.Type,
				Unit:       metric.Unit,
				Value:      metric.Value,
				ResetDaily: metric.ResetDaily,
			})
		}

		return metricsMsg{
			metrics:    metrics,
			lastUpdate: time.Now(),
		}
	}
}

// actionCompletedMsg signals that an action has been successfully completed.
type actionCompletedMsg struct {
	action string // The action that was completed (e.g., "add", "inc")
}

// addMetric sends a request to add a new metric.
func (m model) addMetric(name, typ, unit string, resetDaily bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := &pb.AddMetricRequest{
			MetricName: name,
			Type:       typ,
			Unit:       unit,
			ResetDaily: resetDaily,
		}

		resp, err := m.client.AddMetric(ctx, req)
		if err != nil {
			return errMsg{err}
		}

		if !resp.Success {
			return errMsg{fmt.Errorf(resp.Message)}
		}

		// After adding, fetch the updated metrics list
		return actionCompletedMsg{action: "add"}
	}
}

func (m model) delMetric(name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := &pb.DeleteMetricRequest{
			MetricName: name,
		}

		resp, err := m.client.DeleteMetric(ctx, req)
		if err != nil {
			return errMsg{err}
		}

		if !resp.Success {
			return errMsg{fmt.Errorf(resp.Message)}
		}

		// After adding, fetch the updated metrics list
		return actionCompletedMsg{action: "del"}
	}
}

func (m model) incrementMetric(name string, value float64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := &pb.IncrementMetricRequest{
			MetricName: name,
			Increment:  value,
		}
		resp, err := m.client.IncrementMetric(ctx, req)

		if err != nil {
			return errMsg{err}
		}

		if !resp.Success {
			return errMsg{fmt.Errorf(resp.Message)}
		}

		// After modification, fetch the updated metrics list
		return actionCompletedMsg{action: "increment"}
	}
}
func (m model) decrementMetric(name string, value float64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := &pb.DecrementMetricRequest{
			MetricName: name,
			Decrement:  value,
		}
		resp, err := m.client.DecrementMetric(ctx, req)

		if err != nil {
			return errMsg{err}
		}

		if !resp.Success {
			return errMsg{fmt.Errorf(resp.Message)}
		}

		// After modification, fetch the updated metrics list
		return actionCompletedMsg{action: "decrement"}
	}
}

// updateMetric sends a request to update a metric's value.
func (m model) updateMetric(name string, newValue float64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		req := &pb.UpdateMetricRequest{
			MetricName: name,
			NewValue:   newValue,
		}

		resp, err := m.client.UpdateMetric(ctx, req)
		if err != nil {
			return errMsg{err}
		}

		if !resp.Success {
			return errMsg{fmt.Errorf(resp.Message)}
		}

		// After updating, fetch the updated metrics list
		return actionCompletedMsg{action: "update"}
	}
}

func newItemDelegate(keys *delegateKeyMap) list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	d.Styles.SelectedTitle = selectedTitleStyle
	d.Styles.SelectedDesc = selectedDescStyle
	d.UpdateFunc = func(msg tea.Msg, m *list.Model) tea.Cmd {
		var title string

		if i, ok := m.SelectedItem().(Metric); ok {
			title = i.Title()
		} else {
			return nil
		}

		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch {
			case key.Matches(msg, keys.choose):
				return m.NewStatusMessage(statusMessageStyle("You chose " + title))
			}
		}

		return nil
	}

	help := []key.Binding{keys.choose}

	d.ShortHelpFunc = func() []key.Binding {
		return help
	}

	d.FullHelpFunc = func() [][]key.Binding {
		return [][]key.Binding{help}
	}

	return d
}

type delegateKeyMap struct {
	choose key.Binding
}

// Additional short help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		d.choose,
	}
}

// Additional full help entries. This satisfies the help.KeyMap interface and
// is entirely optional.
func (d delegateKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{
			d.choose,
		},
	}
}

func newDelegateKeyMap() *delegateKeyMap {
	return &delegateKeyMap{
		choose: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "choose"),
		),
	}
}

// =============================================================
// Main Function
// =============================================================

func main() {

	serverAddr := flag.String("server", "localhost:50051", "gRPC server address in the format ip:port")
	flag.Parse()

	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Establish gRPC connection
	conn, err := grpc.NewClient(*serverAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	// Initialize gRPC client
	client := pb.NewMetricsServiceClient(conn)

	if _, err := tea.NewProgram(initialModel(client), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
