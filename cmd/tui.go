/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

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
	keywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	checkboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	progressEmpty = subtleStyle.Render(progressEmptyChar)
	dotStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(dotChar)
	mainStyle     = lipgloss.NewStyle().MarginLeft(2)

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
}

type configWrittenMsg struct{}

type errMsg struct {
	Err error
}

func initialModel() model {

	configExist := false

	//if config file exists, set configExist to true
	//set to false for testing
	if _, err := os.Stat("config.json"); err == nil {
		configExist = false
		//pull values out of config file and set them to the model
		configData, err := os.ReadFile("config.json")
		if err != nil {
			fmt.Println("Error reading config file:", err)
		}
		var configVars struct {
			PwndocUrl        string
			OutputDir        string
			ScoutSuiteReport string
			AwsAccessKey     string
			AwsSecretKey     string
			AwsToken         string
		}
		//assign from json
		err = json.Unmarshal(configData, &configVars)
		if err != nil {
			fmt.Println("Error unmarshalling config file:", err)
		}
		//assign to model
		return model{
			Choice:      0,
			Chosen:      false,
			Frames:      0,
			Progress:    0,
			Loaded:      false,
			Quitting:    false,
			ConfigExist: configExist,
			Config:      0,
			ConfigVars:  configVars,
		}
	}
	return model{
		Choice:      0,
		Chosen:      false,
		Frames:      0,
		Progress:    0,
		Loaded:      false,
		Quitting:    false,
		ConfigExist: configExist,
		Config:      0,
		ConfigVars: struct {
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
		},
	}
}

func startTui() {

	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Println("could not start program:", err)
	}
}

func (m model) Init() tea.Cmd {
	return tick()
}

// Main update function.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Make sure these keys always quit
	if msg, ok := msg.(tea.KeyMsg); ok {
		k := msg.String()
		if k == "q" || k == "esc" || k == "ctrl+c" {
			m.Quitting = true
			return m, tea.Quit
		}
	}

	// Hand off the message and model to the appropriate update function for the
	// appropriate view based on the current state.

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
	if !m.ConfigExist {
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

			if msg.String() == "enter" && !m.ConfigExist {
				m.ConfigExist = true
				//initiate writing idk if this is in the optimal place
				return m, writeConfig(m)
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

	tpl := "What to do today?\n\n"
	tpl += "%s\n\n"
	tpl += subtleStyle.Render("j/k, up/down: select") + dotStyle +
		subtleStyle.Render("enter: choose") + dotStyle +
		subtleStyle.Render("q, esc: quit")

	choices := fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		checkbox("Plant carrots", c == 0),
		checkbox("Go to the market", c == 1),
		checkbox("Read something", c == 2),
		checkbox("See friends", c == 3),
	)

	return fmt.Sprintf(tpl, choices)
}

// The second view, after a task has been chosen
func chosenView(m model) string {
	var msg string

	switch m.Choice {
	case 0:
		msg = fmt.Sprintf("Carrot planting?\n\nCool, we'll need %s and %s...", keywordStyle.Render("libgarden"), keywordStyle.Render("vegeutils"))
	case 1:
		msg = fmt.Sprintf("A trip to the market?\n\nOkay, then we should install %s and %s...", keywordStyle.Render("marketkit"), keywordStyle.Render("libshopping"))
	case 2:
		msg = fmt.Sprintf("Reading time?\n\nOkay, cool, then we’ll need a library. Yes, an %s.", keywordStyle.Render("actual library"))
	default:
		msg = fmt.Sprintf("It’s always good to see friends.\n\nFetching %s and %s...", keywordStyle.Render("social-skills"), keywordStyle.Render("conversationutils"))
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
