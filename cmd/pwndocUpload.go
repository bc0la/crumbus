package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/strykethru/pwndoctor/pkg/pwndoc"
	"github.com/strykethru/pwndoctor/pkg/pwndoctor"
	"github.com/strykethru/pwndoctor/pkg/util"
)

func PwndocUploader(m *model) tea.Cmd {
	return func() tea.Msg {
		// Upload the pwndoc to the server
		m.moduleDebugChan <- DebugMsg{"Uploading pwndoc to server"}
		pwndocApi := InitPwndocAPI(m.ConfigVars.PwndocUrl)
		allAudits, err := pwndocApi.GetAudits()
		if err != nil {
			m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Unable to retrieve audits: %s", err.Error())}
			time.Sleep(5 * time.Second)
		}

		// Print the audits
		// Auditnames := GetAuditNames(retrievedAudits)
		// More robust audit selection should be added to a config view early on
		for _, audit := range allAudits.Data {
			if audit.Name == m.ConfigVars.PwndocAuditName && !m.doneUploading {

				// Upload the attack narrative to the server
				for _, module := range m.PwndocModules {
					if module.Selected && module.AffectedAssets != nil && len(module.AffectedAssets) > 0 {
						go uploadAttackNarrative(*pwndocApi, m, audit, module)

					}
				}

				//need to figure out where to set m.doneUploading to true

			}

		}

		for _, module := range m.PwndocModules {
			if module.Selected && module.Name == "Access Key Age/Last Used" {

				// send an update every second for 10 seconds
				for i := 0; i < 10; i++ {
					m.moduleProgressChan <- ModuleProgressMsg{ModuleName: module.Name, Checked: i + 1, Total: 10}
					time.Sleep(1 * time.Second)
				}
				m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: module.Name}
			}

			if module.Selected && module.Name == "Open S3 Buckets (Authenticated/Anonymous)" {
				// send an update every second for 10 seconds
				for i := 0; i < 10; i++ {
					m.moduleProgressChan <- ModuleProgressMsg{ModuleName: module.Name, Checked: i + 1, Total: 10}
					time.Sleep(1 * time.Second)
				}
				m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: module.Name}

			}
		}
		return nil

	}

}

func uploadAttackNarrative(pwndocApi pwndoc.API, m *model, audit pwndoc.APIAudit, currentModule Module) {
	// Upload the attack narrative to the server
	m.moduleDebugChan <- DebugMsg{"Uploading attack narrative to  " + audit.Name}

	retrievedAuditInformation, err := pwndocApi.GetAudit(audit.ID)
	if err != nil {
		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Unable to retrieve audit information: %s", err.Error())}
		time.Sleep(3 * time.Second)
	}

	for _, section := range retrievedAuditInformation.Data.Sections {
		m.moduleDebugChan <- DebugMsg{"Checking section " + section.Name}
		//time.Sleep(1 * time.Second)
		//should be a configvar
		if section.Name == "Cloud" {
			m.moduleDebugChan <- DebugMsg{"Found Cloud section " + audit.Name}
			time.Sleep(1 * time.Second)
			for i, field := range section.CustomFields {
				//should be a configvar
				if field.CustomField.Label == "Cloud Narrative" {
					text, ok := field.Text.(string)
					if !ok {
						m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Expected string for field.Text, got %T", field.Text)}
						time.Sleep(3 * time.Second)
						continue
					}
					m.moduleDebugChan <- DebugMsg{"Found attack narrative: " + audit.Name}
					time.Sleep(1 * time.Second)
					// Search and replace in the text
					searchString := currentModule.AffectedAmountSearchString
					assetString := currentModule.AffectedAssetsSearchString

					replacementAssets := ""
					for _, asset := range currentModule.AffectedAssets {
						replacementAssets += ("<li><p>" + asset.Name + ": " + asset.ID + "</p></li>\n")
					}

					replacement := strconv.Itoa(len(currentModule.AffectedAssets))

					updatedText := strings.ReplaceAll(text, searchString, replacement)
					updatedAssets := strings.ReplaceAll(updatedText, assetString, replacementAssets)

					// Assign the updated text back to the field
					section.CustomFields[i].Text = updatedText
					section.CustomFields[i].Text = updatedAssets

					// Update the audit with the new field
					url := fmt.Sprintf("/api/audits/%s/sections/%s", audit.ID, section.ID)
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Updating attack narrative at %s", url)}
					time.Sleep(1 * time.Second)
					bodyReader, err := util.MarshalStuff(section)
					if err != nil {
						m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error marshalling section: %s", err.Error())}
						time.Sleep(3 * time.Second)
						continue
					}
					m.moduleDebugChan <- DebugMsg{Message: ("Pre suppress stdout: " + audit.Name)}
					time.Sleep(1 * time.Second)

					err = nil
					suppressStdout(func() {
						body, err := pwndocApi.PutResponseBody(url, bodyReader)
						if err != nil {
							m.moduleDebugChan <- DebugMsg{fmt.Sprintf("error uploading: %s", err.Error())}
							time.Sleep(3 * time.Second)

						}
						//log the response to a file
						f, err := os.Create("response.txt")
						if err != nil {
							m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error creating file: %s", err.Error())}

						}
						defer f.Close()
						_, err = f.Write(body)
						if err != nil {
							m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error writing to file: %s", err.Error())}
						}
					})

					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Updated attack narrative: %s, %s, %s", section.Name, searchString, replacement)}
					time.Sleep(6 * time.Second)
					if err != nil {
						m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error updating attack narrative: %s", err.Error())}
						time.Sleep(3 * time.Second)
						continue
					}
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Updated attack narrative: %s", section.Name)}
				}
			}
		}
	}

	// Progress and completion notification
	m.moduleProgressChan <- ModuleProgressMsg{ModuleName: "Attack Narrative", Checked: currentModule.Checked + 1, Total: 10}
	time.Sleep(1 * time.Second)
	m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: "Attack Narrative"}
}

// suppressStdout temporarily redirects stdout to a buffer and restores it afterwards.
// This is a hacky way to stop the libs from printing to muh stdout
func suppressStdout(f func()) {
	// Keep backups of the real stdout and stderr
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	// Create new pipes
	stdoutR, stdoutW, _ := os.Pipe()
	stderrR, stderrW, _ := os.Pipe()
	os.Stdout = stdoutW
	os.Stderr = stderrW

	// Run the function with stdout and stderr suppressed
	f()

	// Close the write ends of the pipes to finish reading
	stdoutW.Close()
	stderrW.Close()

	// Optionally, read from the pipes to capture the suppressed output
	var stdoutBuf, stderrBuf bytes.Buffer
	io.Copy(&stdoutBuf, stdoutR)
	io.Copy(&stderrBuf, stderrR)

	// You could use the captured output if needed
	// fmt.Println("Captured stdout:", stdoutBuf.String())
	// fmt.Println("Captured stderr:", stderrBuf.String())
}

func InitPwndocAPI(endpoint string) *pwndoc.API {
	var pwndocAPI *pwndoc.API
	suppressStdout(func() {
		pwndoctor.Init(endpoint)
		pwndoctor.AutoAuth()
		pwndocAPI = pwndoctor.GetPwndocAPI()
	})

	return pwndocAPI
}
