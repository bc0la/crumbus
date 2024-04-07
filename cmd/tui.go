/*
Made with *<3* by cola
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fogleman/ease"
	"github.com/lucasb-eyer/go-colorful"
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
	keywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	progressEmpty = subtleStyle.Render(progressEmptyChar)
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

	// Gradient colors we'll use for the progress bar
	ramp = makeRampStyles("#B14FFF", "#00FFA3", progressBarWidth)
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
	}
}

type model struct {
	Choice int
	Chosen bool

	Frames      int
	Progress    float64
	Loaded      bool
	Quitting    bool
	ConfigExist bool
	Config      int
	ConfigVars  struct {
		PwndocUrl        string
		OutputDir        string
		ScoutSuiteReport string
		AwsAccessKey     string
		AwsSecretKey     string
		AwsToken         string
	}
	// textinput
	focusIndex int
	inputs     []textinput.Model
	cursorMode cursor.Mode
	Configured bool
}

type configWrittenMsg struct{}

type errMsg struct {
	Err error
}

func initialModel() model {

	configExist := false
	var configVars = struct {
		PwndocUrl        string
		OutputDir        string
		ScoutSuiteReport string
		AwsAccessKey     string
		AwsSecretKey     string
		AwsToken         string
	}{
		PwndocUrl:        "https://192.168.1.51:8443",
		OutputDir:        "/home/username/crumbus/outputs",
		ScoutSuiteReport: "/home/username/crumbus/scout/scoutsuite-report.html",
		AwsAccessKey:     "",
		AwsSecretKey:     "",
		AwsToken:         "",
	}

	//if config file exists, set configExist to true
	//set to false for testing
	if _, err := os.Stat("config.json"); err == nil {
		configExist = false
		//pull values out of config file and set them to the model
		configData, err := os.ReadFile("config.json")
		if err != nil {
			fmt.Println("Error reading config file:", err)
		}

		//assign from json
		err = json.Unmarshal(configData, &configVars)
		if err != nil {
			fmt.Println("Error unmarshalling config file:", err)
		}

	}

	m := model{
		Choice:      0,
		Chosen:      false,
		Frames:      0,
		Progress:    0,
		Loaded:      false,
		Quitting:    false,
		ConfigExist: configExist,
		Config:      0,
		ConfigVars:  configVars,
		inputs:      make([]textinput.Model, reflect.TypeOf(configVars).NumField()),
		Configured:  false,
	}
	//textinput bits
	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 32

		switch i {
		case 0:
			t.Placeholder = "PwnDocURL: " + m.ConfigVars.PwndocUrl
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "OutPutDir: " + m.ConfigVars.OutputDir
		case 2:
			t.Placeholder = "ScoutSuiteReport: " + m.ConfigVars.ScoutSuiteReport
		case 3:
			t.Placeholder = "AWSAccessKey: " + m.ConfigVars.AwsAccessKey
		case 4:
			t.Placeholder = "AWSSecretKey: " + m.ConfigVars.AwsSecretKey
		case 5:
			t.Placeholder = "AWSToken: " + m.ConfigVars.AwsToken

		}

		m.inputs[i] = t
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
		if k == "q" {
			if m.Configured {
				m.Quitting = true
				return m, tea.Quit
			}
		}
	}
	//this should maybe be in the subupdate function

	if !m.Configured {
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

					m.Configured = true

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

				return m, tea.Batch(cmds...)
			}
		}

		// Handle character input and blinking
		if !m.Configured {
			cmd := m.updateConfiguration(msg)

			return m, cmd
		}
	}
	// Hand off the message and model to the appropriate update function for the
	// appropriate view based on the current state.

	// perhaps add a "would you like to configure the tool or verify the configuration?" view
	if !m.ConfigExist {
		return updateConfig(msg, m)
	}
	if !m.Chosen {
		return updateChoices(msg, m)
	}
	return updateChosen(msg, m)
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
	} else if !m.ConfigExist {
		s = configView(m)
	} else if !m.Chosen {
		s = choicesView(m)
	} else {
		s = chosenView(m)
	}
	return mainStyle.Render("\n" + s + "\n\n")
}

// Sub-update functions
// Update loop for the first view where you're choosing a task.
// need to adjust this and the view to present a config wizard

func (m *model) updateConfiguration(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only text inputs with Focus() set will respond, so it's safe to simply
	// update all of them here without any further logic.
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}

	return tea.Batch(cmds...)
}

func updateConfig(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
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
			// if msg.String() == "enter" && !m.ConfigExist
			if msg.String() == "enter" {
				m.ConfigExist = true
				//initiate writing idk if this is in the optimal place
				//this will need to happen on a specific selection in the config wizard (like a confirm button)
				return m, frame()
			}
			return m, frame()
		}
		// Example: on pressing enter, initiate config writing

	// Handle the completion of the config writing
	case configWrittenMsg:
		// Update state to reflect the config has been written
		m.ConfigExist = true
		// Return model and nil or another command if needed
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

	return m, nil
}

// Update loop for the first view where you're choosing a task.
func updateChoices(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			m.Choice++
			if m.Choice > 3 {
				m.Choice = 3
			}
		case "k", "up":
			m.Choice--
			if m.Choice < 0 {
				m.Choice = 0
			}
		case "enter":
			m.Chosen = true
			return m, frame()
		}

	}

	return m, nil
}

// Update loop for the second view after a choice has been made
func updateChosen(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case frameMsg:
		if !m.Loaded {
			m.Frames++
			m.Progress = ease.OutBounce(float64(m.Frames) / float64(100))
			if m.Progress >= 1 {
				m.Progress = 1
				m.Loaded = true

				return m, tick()
			}
			return m, frame()
		}

	}

	return m, nil
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
func configView(m model) string {
	c := m.Config

	tpl := "This is the mfin config view\n\n"
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

// where you're choosing a task
func choicesView(m model) string {
	c := m.Choice

	tpl := "Choices view\n\n"
	tpl += "%s\n\n"
	tpl += subtleStyle.Render("j/k, up/down: select") + dotStyle +
		subtleStyle.Render("enter: choose") + dotStyle +
		subtleStyle.Render("q, esc: quit")

	choices := fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		checkbox("choice1", c == 0),
		checkbox("choice2", c == 1),
		checkbox("choice3", c == 2),
		checkbox("choice4", c == 3),
	)

	return fmt.Sprintf(tpl, choices)
}

// The second view, after a task has been chosen
func chosenView(m model) string {
	var msg string

	switch m.Choice {
	case 0:
		msg = fmt.Sprintf("Cool...\n\nThis is a %s progress bar", keywordStyle.Render("fake"))
	case 1:
		msg = fmt.Sprintf("A trip to the market?\n\nOkay, then we should install %s and %s...", keywordStyle.Render("marketkit"), keywordStyle.Render("libshopping"))
	case 2:
		msg = fmt.Sprintf("Reading time?\n\nOkay, cool, then we'll need a library. Yes, an %s.", keywordStyle.Render("actual library"))
	default:
		msg = fmt.Sprintf("It's always good to see friends.\n\nFetching %s and %s...", keywordStyle.Render("social-skills"), keywordStyle.Render("conversationutils"))
	}

	label := "Downloading..."
	if m.Loaded {
		label = "Downloaded. Press q to exit ..."
	}

	return msg + "\n\n" + label + "\n" + progressbar(m.Progress) + "%"
}

func checkbox(label string, checked bool) string {
	if checked {
		return checkboxStyle.Render("[x] " + label)
	}
	return fmt.Sprintf("[ ] %s", label)
}

func progressbar(percent float64) string {
	w := float64(progressBarWidth)

	fullSize := int(math.Round(w * percent))
	var fullCells string
	for i := 0; i < fullSize; i++ {
		fullCells += ramp[i].Render(progressFullChar)
	}

	emptySize := int(w) - fullSize
	emptyCells := strings.Repeat(progressEmpty, emptySize)

	return fmt.Sprintf("%s%s %3.0f", fullCells, emptyCells, math.Round(percent*100))
}

// tea cmds
func updateConfigVars(m model) tea.Cmd {
	return func() tea.Msg {
		// Create a copy of the current ConfigVars to modify
		updatedConfigVars := m.ConfigVars

		// Update each field only if the corresponding input is not empty
		if val := m.inputs[0].Value(); val != "" {
			updatedConfigVars.PwndocUrl = val
		}
		if val := m.inputs[1].Value(); val != "" {
			updatedConfigVars.OutputDir = val
		}
		if val := m.inputs[2].Value(); val != "" {
			updatedConfigVars.ScoutSuiteReport = val
		}
		if val := m.inputs[3].Value(); val != "" {
			updatedConfigVars.AwsAccessKey = val
		}
		if val := m.inputs[4].Value(); val != "" {
			updatedConfigVars.AwsSecretKey = val
		}
		if val := m.inputs[5].Value(); val != "" {
			updatedConfigVars.AwsToken = val
		}

		// Return a message with the updated configuration variables
		return configUpdatedMsg{
			ConfigVars: updatedConfigVars,
		}
	}
}
func writeConfig(m model) tea.Cmd {
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

// Generate a blend of colors.
func makeRampStyles(colorA, colorB string, steps float64) (s []lipgloss.Style) {
	cA, _ := colorful.Hex(colorA)
	cB, _ := colorful.Hex(colorB)

	for i := 0.0; i < steps; i++ {
		c := cA.BlendLuv(cB, i/steps)
		s = append(s, lipgloss.NewStyle().Foreground(lipgloss.Color(colorToHex(c))))
	}
	return
}

// Convert a colorful.Color to a hexadecimal format.
func colorToHex(c colorful.Color) string {
	return fmt.Sprintf("#%s%s%s", colorFloatToHex(c.R), colorFloatToHex(c.G), colorFloatToHex(c.B))
}

// Helper function for converting colors to hex. Assumes a value between 0 and
// 1.
func colorFloatToHex(f float64) (s string) {
	s = strconv.FormatInt(int64(f*255), 16)
	if len(s) == 1 {
		s = "0" + s
	}
	return
}

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
		fmt.Println("tui called")
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
