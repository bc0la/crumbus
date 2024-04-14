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

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/spinner"
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

var (
// Available spinners

)

// General stuff for styling the view
var (
	// spinners = []spinner.Spinner{
	// 	spinner.Line,
	// 	spinner.Dot,
	// 	spinner.MiniDot,
	// 	spinner.Jump,
	// 	spinner.Pulse,
	// 	spinner.Points,
	// 	spinner.Globe,
	// 	spinner.Moon,
	// 	spinner.Monkey,
	// }

	// textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("69"))

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
	highlightStyle      = lipgloss.NewStyle().Background(lipgloss.Color("205"))

	focusedButton = focusedStyle.Copy().Render("[ Submit ]")
	blurredButton = fmt.Sprintf("[ %s ]", blurredStyle.Render("Submit"))
)

// type (
// 	tickMsg struct{}
// 	//frameMsg struct{}
// )

// func tick() tea.Cmd {
// 	return tea.Tick(time.Second, func(time.Time) tea.Msg {
// 		return tickMsg{}
// 	})
// }

// func frame() tea.Cmd {
// 	return tea.Tick(time.Second/60, func(time.Time) tea.Msg {
// 		return frameMsg{}
// 	})
// }

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

type Module struct {
	Name     string
	Selected bool
}

type model struct {
	// are we quitting?
	Quitting bool
	// int for module selection, need to rename
	Module int
	// ModuleSelection map
	ModuleSelection    []Module
	currentModuleIndex int

	// Pwndoc Check Modules
	PwndocModules      []Module
	currentPwndocIndex int
	// Non Pwndoc Check Modules
	NonPwndocModules      []Module
	currentNonPwndocIndex int

	// Guided Check Modules
	GuidedCheckModules      []Module
	currentGuidedCheckIndex int

	// Have we completed ModuleSelection?
	ModuleSelected bool
	// Have we completed the configuration wizard?
	Configured bool
	// Are we reviewing submodules?
	SubModulesReviewed bool
	Executing          bool
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

	//spinner
	spinner spinner.Model
	//index   int
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

	s := spinner.New()
	s.Spinner = spinner.Line
	s.Style = spinnerStyle
	m := model{
		Quitting:           false,
		Module:             0,
		ConfigVars:         configVars,
		inputs:             make([]textinput.Model, reflect.TypeOf(configVars).NumField()),
		Configured:         false,
		ModuleSelected:     false,
		SubModulesReviewed: false,
		Executing:          false,
		ModuleSelection: []Module{
			{Name: "Pwndoc Checks", Selected: true},
			{Name: "Non Pwndoc Checks", Selected: true},
			{Name: "Guided Checks", Selected: true},
		},
		PwndocModules: []Module{
			{Name: "Access Key Age/Last Used", Selected: false},
			{Name: "Open S3 Buckets (Authenticated/Anonymous)", Selected: false},
			{Name: "IMDSv1", Selected: false},
			{Name: "Public RDS", Selected: false},
			{Name: "Unencrypted EBS Snapshots", Selected: false},
			{Name: "RDS Minor Version Upgrade (Informational)", Selected: false},
			{Name: "Root Account in Use", Selected: false},
		},
		NonPwndocModules: []Module{
			{Name: "cf-template-* S3/Cloudformation template injection", Selected: false},
			{Name: "Pull Lambda Source", Selected: false},
			{Name: "Pull Lambda Env Variables", Selected: false},
			{Name: "S3 List/Recon", Selected: false},
		},
		GuidedCheckModules: []Module{
			{Name: "SQS Queues", Selected: false},
			{Name: "IAM Stuff", Selected: false},
		},

		currentModuleIndex:      0,
		currentPwndocIndex:      0,
		currentNonPwndocIndex:   0,
		currentGuidedCheckIndex: 0,

		spinner: s,
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

	return m.spinner.Tick
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
	} else if !m.ModuleSelected {
		return moduleSelection(msg, m)
	} else if m.ModuleSelection[0].Selected {
		m, cmd := m.updatePwndocChecks(msg)
		return m, cmd
	} else if m.ModuleSelection[1].Selected {
		m, cmd := m.updateNonPwndocChecks(msg)
		return m, cmd
	} else if m.ModuleSelection[2].Selected {
		m, cmd := m.updateGuidedChecks(msg)
		return m, cmd
	} else if !m.SubModulesReviewed {
		m, cmd := m.updateReviewSubModules(msg)
		return m, cmd
	} else if m.Executing {
		m, cmd := m.updateExecuteChecks(msg)
		return m, cmd
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

		//write the configuration to file or any other next step
		return m, writeConfig(m)

	case errMsg:
		// Handle the error, maybe log it or display an error message in the UI
		fmt.Println("Error writing config:", msg.Err)
		return m, nil

	case spinner.TickMsg:
		if m.Executing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
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
			prevFocusIndex := m.focusIndex
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

			// After updating focusIndex, check if focus has changed. This seems a bit hacky but oh well
			if prevFocusIndex != m.focusIndex {
				// Focus has changed, ensure the newly focused input's cursor blinks
				cmds = append(cmds, textinput.Blink)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			cmds = append(cmds, m.updateInputs(msg))
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
	//	cmds = append(cmds, textinput.Blink)

	return m, tea.Batch(cmds...)
}

// Update loop for the moduleSelection view
func moduleSelection(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	// Variable to store commands to execute
	var cmds []tea.Cmd

	// Handling key inputs
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			// Move the selection down
			m.currentModuleIndex = (m.currentModuleIndex + 1) % len(m.ModuleSelection)
			return m, nil
		case "k", "up":
			// Move the selection up
			m.currentModuleIndex = (m.currentModuleIndex - 1 + len(m.ModuleSelection)) % len(m.ModuleSelection)
			return m, nil
		case " ":
			// Toggle the selected state of the current module
			m.ModuleSelection[m.currentModuleIndex].Selected = !m.ModuleSelection[m.currentModuleIndex].Selected
			return m, nil
		case "enter":
			// Confirm selections and possibly move to the next state
			if m.ModuleSelection[0].Selected {
				//set all pwndoc modules to selected
				for i := range m.PwndocModules {
					m.PwndocModules[i].Selected = true
				}
			}
			if m.ModuleSelection[1].Selected {
				//set all non pwndoc modules to selected
				for i := range m.NonPwndocModules {
					m.NonPwndocModules[i].Selected = true
				}
			}
			if m.ModuleSelection[2].Selected {
				//set all guided check modules to selected
				for i := range m.GuidedCheckModules {
					m.GuidedCheckModules[i].Selected = true
				}
			}

			m.ModuleSelected = true
			return m, nil
		case "q", "esc":
			// Exit module selection
			m.Quitting = true
			return m, tea.Quit
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *model) updatePwndocChecks(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handling key inputs specific to navigating and toggling Pwndoc checks
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			// Navigate down through the list of Pwndoc checks
			m.currentPwndocIndex = (m.currentPwndocIndex + 1) % len(m.PwndocModules)
		case "k", "up":
			// Navigate up through the list of Pwndoc checks
			m.currentPwndocIndex = (m.currentPwndocIndex - 1 + len(m.PwndocModules)) % len(m.PwndocModules)
		case " ":
			// Toggle the selected state of the currently highlighted Pwndoc module
			m.PwndocModules[m.currentPwndocIndex].Selected = !m.PwndocModules[m.currentPwndocIndex].Selected
		case "enter":
			m.ModuleSelection[0].Selected = false
			return m, nil
		case "q", "esc":
			// Exit the Pwndoc checks update
			return m, tea.Quit
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *model) updateNonPwndocChecks(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handling key inputs specific to navigating and toggling Non Pwndoc checks
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			// Navigate down through the list of Non Pwndoc checks
			m.currentNonPwndocIndex = (m.currentNonPwndocIndex + 1) % len(m.NonPwndocModules)
		case "k", "up":
			// Navigate up through the list of Non Pwndoc checks

			m.currentNonPwndocIndex = (m.currentNonPwndocIndex - 1 + len(m.NonPwndocModules)) % len(m.NonPwndocModules)
		case " ":
			// Toggle the selected state of the currently highlighted Non Pwndoc module
			m.NonPwndocModules[m.currentNonPwndocIndex].Selected = !m.NonPwndocModules[m.currentNonPwndocIndex].Selected
		case "enter":
			m.ModuleSelection[1].Selected = false

			return m, nil
		case "q", "esc":
			// Exit the Non Pwndoc checks update
			return m, tea.Quit
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *model) updateGuidedChecks(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handling key inputs specific to navigating and toggling Guided checks
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			// Navigate down through the list of Guided checks
			m.currentGuidedCheckIndex = (m.currentGuidedCheckIndex + 1) % len(m.GuidedCheckModules)
		case "k", "up":
			// Navigate up through the list of Guided checks
			m.currentGuidedCheckIndex = (m.currentGuidedCheckIndex - 1 + len(m.GuidedCheckModules)) % len(m.GuidedCheckModules)
		case " ":
			// Toggle the selected state of the currently highlighted Guided module
			m.GuidedCheckModules[m.currentGuidedCheckIndex].Selected = !m.GuidedCheckModules[m.currentGuidedCheckIndex].Selected
		case "enter":
			m.ModuleSelection[2].Selected = false

			return m, nil
		case "q", "esc":
			// Exit the Guided checks update
			return m, tea.Quit
		}
	}
	return m, tea.Batch(cmds...)
}

func (m *model) updateReviewSubModules(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Display the selected modules

	// Handle key inputs
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":

			m.SubModulesReviewed = true
			m.Executing = true

			return m, m.spinner.Tick
		case "q", "esc":
			// Exit
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *model) updateExecuteChecks(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	// Handle other messages or commands specific to execution
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
	} else if m.ModuleSelection[0].Selected {
		s = pwndocChecksView(m)
	} else if m.ModuleSelection[1].Selected {
		s = nonPwndocChecksView(m)
	} else if m.ModuleSelection[2].Selected {
		s = guidedChecksView(m)
	} else if !m.SubModulesReviewed {
		s = subModulesReviewView(m)
	} else if m.Executing {
		s = executionView(m)
	} else {
		s = "Press any key to exit"
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
	var b strings.Builder
	b.WriteString("Select Modules:\n\n")

	for i, mod := range m.ModuleSelection {
		// Highlight the current module
		line := fmt.Sprintf("[ ] %s", mod.Name)
		if mod.Selected {
			line = fmt.Sprintf("[x] %s", mod.Name)
		}
		if i == m.currentModuleIndex {
			line = highlightStyle.Render(line) // Apply highlight style
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(subtleStyle.Render("\nUse j/k or up/down to navigate") + dotStyle +
		subtleStyle.Render("space to toggle selection") + dotStyle +
		subtleStyle.Render("enter to confirm") + dotStyle +
		subtleStyle.Render("esc to quit"))
	return b.String()
}

func pwndocChecksView(m model) string {
	var b strings.Builder
	b.WriteString("Pwndoc Module Checks:\n\n")

	for i, mod := range m.PwndocModules {
		// Create a checkbox line for each module
		line := fmt.Sprintf("[ ] %s", mod.Name)
		if mod.Selected {
			line = fmt.Sprintf("[x] %s", mod.Name)
		}
		if i == m.currentPwndocIndex {
			line = highlightStyle.Render(line) // Apply highlight style to the current index
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(subtleStyle.Render("\nUse j/k or up/down to navigate") + dotStyle +
		subtleStyle.Render("space to toggle selection") + dotStyle +
		subtleStyle.Render("enter to confirm") + dotStyle +
		subtleStyle.Render("q, esc to quit"))
	return b.String()
}

func nonPwndocChecksView(m model) string {
	var b strings.Builder
	b.WriteString("Non Pwndoc Module Checks:\n\n")

	for i, mod := range m.NonPwndocModules {
		// Create a checkbox line for each module
		line := fmt.Sprintf("[ ] %s", mod.Name)
		if mod.Selected {
			line = fmt.Sprintf("[x] %s", mod.Name)
		}
		if i == m.currentNonPwndocIndex {
			line = highlightStyle.Render(line) // Apply highlight style to the current index
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(subtleStyle.Render("\nUse j/k or up/down to navigate") + dotStyle +

		subtleStyle.Render("space to toggle selection") + dotStyle +
		subtleStyle.Render("enter to confirm") + dotStyle +
		subtleStyle.Render("q, esc to quit"))
	return b.String()
}

func guidedChecksView(m model) string {
	var b strings.Builder
	b.WriteString("Guided Check Modules:\n\n")

	for i, mod := range m.GuidedCheckModules {
		// Create a checkbox line for each module
		line := fmt.Sprintf("[ ] %s", mod.Name)
		if mod.Selected {
			line = fmt.Sprintf("[x] %s", mod.Name)
		}
		if i == m.currentGuidedCheckIndex {
			line = highlightStyle.Render(line) // Apply highlight style to the current index
		}
		b.WriteString(line + "\n")
	}

	b.WriteString(subtleStyle.Render("\nUse j/k or up/down to navigate") + dotStyle +

		subtleStyle.Render("space to toggle selection") + dotStyle +
		subtleStyle.Render("enter to confirm") + dotStyle +
		subtleStyle.Render("q, esc to quit"))
	return b.String()
}

func subModulesReviewView(m model) string {
	var b strings.Builder
	b.WriteString("Review Selected Modules:\n\n")
	b.WriteString("Pwndoc Checks:\n")
	for _, mod := range m.PwndocModules {
		// Create a checkbox line for each module
		b.WriteString(checkbox(mod.Name, mod.Selected) + "\n")
	}
	b.WriteString("\nNon Pwndoc Checks:\n")
	for _, mod := range m.NonPwndocModules {
		// Create a checkbox line for each module
		b.WriteString(checkbox(mod.Name, mod.Selected) + "\n")
	}
	b.WriteString("\nGuided Checks:\n")
	for _, mod := range m.GuidedCheckModules {
		// Create a checkbox line for each module
		b.WriteString(checkbox(mod.Name, mod.Selected) + "\n")
	}

	b.WriteString(subtleStyle.Render("\nPress enter to confirm") + dotStyle +

		subtleStyle.Render("q, esc to quit"))
	return b.String()
}

func executionView(m model) string {
	var b strings.Builder
	b.WriteString("\n" + m.spinner.View() + " Executing, please wait...\n")
	return b.String()
}

func checkbox(label string, checked bool) string {
	if checked {
		return checkboxStyle.Render("[x] " + label)
	}
	return fmt.Sprintf("[ ] %s", label)
}

// Update the text inputs
func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

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
