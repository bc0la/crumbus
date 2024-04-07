/*
Made with *<3* by cola
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const (
	progressBarWidth  = 71
	progressFullChar  = "█"
	progressEmptyChar = "░"
	dotChar           = " • "
)

// General stuff for styling the view
var (
	//checkbox styling
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	dotStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(dotChar)
	mainStyle     = lipgloss.NewStyle().MarginLeft(2)

	//textinputs bubble styling
	focusedStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle         = focusedStyle.Copy()
	noStyle             = lipgloss.NewStyle()
	helpStyle           = blurredStyle.Copy()
	cursorModeHelpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

type (
	tickMsg  struct{}
	frameMsg struct{}
)

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func frame() tea.Cmd {
	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
		return frameMsg{}
	})
}

type configUpdatedMsg struct {
	ConfigVars struct {
		PwndocUrl        string
		OutputDir        string
		ScoutSuiteReport string
		AwsAccessKey     string
		AwsSecretKey     string
		AwsToken         string
		TestValue        string
	}
}

type model struct {
	// are we quitting?
	Quitting bool
	// int for module selection, need to rename
	Config int
	// Have we completed ModuleSelection?
	ModuleSelected bool
	// Have we completed the configuration wizard?
	Configured bool
	// Config vars for config.json/tool configuration
	ConfigVars struct {
		PwndocUrl        string
		OutputDir        string
		ScoutSuiteReport string
		AwsAccessKey     string
		AwsSecretKey     string
		AwsToken         string
		TestValue        string
	}
	// textinput
	focusIndex int
	inputs     []textinput.Model
	cursorMode cursor.Mode
}

type configWrittenMsg struct{}

type errMsg struct {
	Err error
}

func initialModel() model {
	var configVars = struct {
		PwndocUrl        string
		OutputDir        string
		ScoutSuiteReport string
		AwsAccessKey     string
		AwsSecretKey     string
		AwsToken         string
		TestValue        string
	}{
		PwndocUrl:        "https://192.168.1.51:8443",
		OutputDir:        "/home/username/crumbus/outputs",
		ScoutSuiteReport: "/home/username/crumbus/scout/scoutsuite-report.html",
		AwsAccessKey:     "",
		AwsSecretKey:     "",
		AwsToken:         "",
		TestValue:        "test",
	}

	// Check if config file exists and load it
	if _, err := os.Stat("config.json"); err == nil {
		configData, err := os.ReadFile("config.json")
		if err != nil {
			fmt.Println("Error reading config file:", err)
		}

		// Unmarshal JSON into configVars
		err = json.Unmarshal(configData, &configVars)
		if err != nil {
			fmt.Println("Error unmarshalling config file:", err)
		}
	}

	m := model{
		Quitting:       false,
		Config:         0,
		ConfigVars:     configVars,
		inputs:         make([]textinput.Model, reflect.TypeOf(configVars).NumField()),
		Configured:     false,
		ModuleSelected: false,
	}

	// Dynamically create text inputs based on the fields of ConfigVars
	t := reflect.TypeOf(configVars)
	v := reflect.ValueOf(configVars)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i).String()

		input := textinput.New()
		input.Cursor.Style = cursorStyle
		input.CharLimit = 100
		input.Placeholder = fmt.Sprintf("%s: %s", field.Name, value)

		if i == 0 {
			input.Focus()
			input.PromptStyle = focusedStyle
			input.TextStyle = focusedStyle
		}

		m.inputs[i] = input
	}

	return m
}

func startTui() {

	// may be better to make multiple models instead of one big one, though the config vars are useful everywhere
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("could not start program:", err)
	}
}

func (m model) Init() tea.Cmd {
	if !m.Configured {
		return textinput.Blink
	}

	return tick()
}

// Main update function.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Make sure these keys always quit IF the user isn't in configView
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		if k == "esc" || k == "ctrl+c" {
			m.Quitting = true
			return m, tea.Quit
		}
		// don't quit on q if we're in the text input configuration view
		if k == "q" {
			if m.Configured {
				m.Quitting = true
				return m, tea.Quit
			}
		}
	}

	// Hand off the message and model to the appropriate update function for the
	// appropriate view based on the current state.
	if !m.Configured {
		m, cmd := m.updateConfiguration(msg)

		return m, cmd
	}
	if !m.ModuleSelected {
		return moduleSelection(msg, m)
	} else {
		return m, tea.Quit
	}

}

// Sub-update functions

// Update loop for the configuration wizard
func (m *model) updateConfiguration(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	// Handle config related updates
	switch msg := msg.(type) {
	case configWrittenMsg:
		// Return model and nil or another command if needed
		m.Configured = true
		return m, nil

	case configUpdatedMsg:
		// Update the model's configuration variables with the new values
		m.ConfigVars = msg.ConfigVars

		// Optionally, proceed to write the configuration to file or any other next step
		return m, writeConfig(m)
	// In your update function
	case errMsg:
		// Handle the error, maybe log it or display an error message in the UI
		fmt.Println("Error writing config:", msg.Err)
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		// Change cursor mode
		case "ctrl+r":
			m.cursorMode++
			if m.cursorMode > cursor.CursorHide {
				m.cursorMode = cursor.CursorBlink
			}
			cmds := make([]tea.Cmd, len(m.inputs))
			for i := range m.inputs {
				cmds[i] = m.inputs[i].Cursor.SetMode(m.cursorMode)
			}
			return m, tea.Batch(cmds...)

		// Set focus to next input
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			// If so,
			if s == "enter" && m.focusIndex == len(m.inputs) {

				return m, updateConfigVars(m)
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
					continue
				}
				// Remove focused state
				m.inputs[i].Blur()
				m.inputs[i].PromptStyle = noStyle
				m.inputs[i].TextStyle = noStyle
			}
		}
	}
	return m, tea.Batch(cmds...)
}

// Update loop for the moduleSelection view
func moduleSelection(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Handle key messages for user inputs
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.Config++
			if m.Config > 3 {
				m.Config = 3
			}
		case "k", "up":
			m.Config--
			if m.Config < 0 {
				m.Config = 0
			}
		case "enter":
			if msg.String() == "enter" {
				m.ModuleSelected = true
				//initiate writing idk if this is in the optimal place
				//this will need to happen on a specific selection in the config wizard (like a confirm button)
				return m, frame()
			}
			return m, frame()
		}
		// Example: on pressing enter, initiate config writing

	}

	return m, nil
}

// The main view, which just calls the appropriate sub-view
func (m model) View() string {
	var s string
	if m.Quitting {
		return "\n  See you later!\n\n"
	}
	// add config file view
	if !m.Configured {
		s = configurationView(m)
	} else if !m.ModuleSelected {
		s = moduleSelectionView(m)
	}
	return mainStyle.Render("\n" + s + "\n\n")
}

// Sub-views
// configuration wizard, need to add logic and create the wizard for item in struct
func configurationView(m model) string {
	var b strings.Builder

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
	}

	button := &blurredButton
	if m.focusIndex == len(m.inputs) {
		button = &focusedButton
	}
	fmt.Fprintf(&b, "\n\n%s\n\n", *button)
	b.WriteString(helpStyle.Render("Leave values empty to keep the current value."))
	b.WriteString(helpStyle.Render("\ncursor mode is "))
	b.WriteString(cursorModeHelpStyle.Render(m.cursorMode.String()))
	b.WriteString(helpStyle.Render(" (ctrl+r to change style)"))
	b.WriteString(helpStyle.Render("\nPress ctrl+c or esc to quit."))

	return b.String()
}
func moduleSelectionView(m model) string {
	c := m.Config

	tpl := "This is the mfin module selector view\n\n"
	tpl += "%s\n\n"
	tpl += subtleStyle.Render("j/k, up/down: select") + dotStyle +
		subtleStyle.Render("enter: choose") + dotStyle +
		subtleStyle.Render("q, esc: quit")

	configs := fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		checkbox("config", c == 0),
		checkbox("configs", c == 1),
		checkbox("poop", c == 2),
		checkbox("stuff", c == 3),
	)

	return fmt.Sprintf(tpl, configs)
}

func checkbox(label string, checked bool) string {
	if checked {
		return checkboxStyle.Render("[x] " + label)
	}
	return fmt.Sprintf("[ ] %s", label)
}

// tea cmds

// Return updated config vars
func updateConfigVars(m *model) tea.Cmd {
	return func() tea.Msg {
		// Get reflect.Value of ConfigVars for setting values
		v := reflect.ValueOf(&m.ConfigVars).Elem()

		for i, input := range m.inputs {
			if val := input.Value(); val != "" {
				// Use field index to set the value dynamically
				v.Field(i).SetString(val)
			}
		}

		// Return a message indicating the update is complete
		// This part remains the same as your current implementation
		return configUpdatedMsg{
			ConfigVars: m.ConfigVars,
		}
	}
}

// Write the config to file
func writeConfig(m *model) tea.Cmd {
	return func() tea.Msg {

		// turn struct into json
		configData, err := json.MarshalIndent(m.ConfigVars, "", "  ")
		if err != nil {
			return errMsg{Err: err}
		}
		err = os.WriteFile("config.json", configData, 0644)
		if err != nil {
			return errMsg{Err: err}
		}
		return configWrittenMsg{}
	}
}

// Utils

// tuiCmd represents the tui command
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		startTui()

	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// tuiCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// tuiCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
