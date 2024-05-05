package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/strykethru/pwndoctor/pkg/pwndoc"
	"github.com/strykethru/pwndoctor/pkg/pwndoctor"
	"github.com/strykethru/pwndoctor/pkg/util"
)

type APIScreenshotUpload struct {
	Value   string `json:"value"`
	Name    string `json:"name"`
	AuditID string `json:"auditID"`
}

type APIScreenshotUploadResponse struct {
	Status string `json:"status"`
	Datas  struct {
		ID string `json:"_id"`
	} `json:"datas"`
}

func PwndocUploader(m *model) tea.Cmd {
	return func() tea.Msg {
		// Upload the pwndoc to the server
		m.moduleDebugChan <- DebugMsg{"Uploading pwndoc to server"}
		pwndocApi := InitPwndocAPI(m.ConfigVars.PwndocUrl)
		allAudits, err := pwndocApi.GetAudits()
		if err != nil {
			m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Unable to retrieve audits: %s", err.Error())}
			//time.Sleep(5 * time.Second)
		}

		// Print the audits
		// Auditnames := GetAuditNames(retrievedAudits)
		// More robust audit selection should be added to a config view early on
		for _, audit := range allAudits.Data {
			if audit.Name == m.ConfigVars.PwndocAuditName && !m.doneUploading {

				// Upload the attack narrative to the server
				for _, module := range m.PwndocModules {
					if module.Selected && module.AffectedAssets != nil && len(module.AffectedAssets) > 0 {
						screenshotString := takeUploadScreenshot(*pwndocApi, m, audit, module)
						m.moduleDebugChan <- DebugMsg{"Screenshot uploaded: " + screenshotString}
						time.Sleep(5 * time.Second)
						go uploadAttackNarrative(*pwndocApi, m, audit, module, screenshotString)
						go createNewFinding(*pwndocApi, m, audit, module, screenshotString)

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
					//time.Sleep(1 * time.Second)
				}
				m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: module.Name}
			}

			if module.Selected && module.Name == "Open S3 Buckets (Authenticated/Anonymous)" {
				// send an update every second for 10 seconds
				for i := 0; i < 10; i++ {
					m.moduleProgressChan <- ModuleProgressMsg{ModuleName: module.Name, Checked: i + 1, Total: 10}
					//time.Sleep(1 * time.Second)
				}
				m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: module.Name}

			}
		}
		return nil

	}

}

func uploadAttackNarrative(pwndocApi pwndoc.API, m *model, audit pwndoc.APIAudit, currentModule Module, screenshotString string) {
	// Upload the attack narrative to the server
	m.moduleDebugChan <- DebugMsg{"Uploading attack narrative to  " + audit.Name}

	retrievedAuditInformation, err := pwndocApi.GetAudit(audit.ID)
	if err != nil {
		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Unable to retrieve audit information: %s", err.Error())}
		//time.Sleep(3 * time.Second)
	}

	for _, section := range retrievedAuditInformation.Data.Sections {
		m.moduleDebugChan <- DebugMsg{"Checking section " + section.Name}
		////time.Sleep(1 * time.Second)
		//should be a configvar
		if section.Name == "Cloud" {
			m.moduleDebugChan <- DebugMsg{"Found Cloud section " + audit.Name}
			//time.Sleep(1 * time.Second)
			for i, field := range section.CustomFields {
				//should be a configvar
				if field.CustomField.Label == "Cloud Narrative" {
					text, ok := field.Text.(string)
					if !ok {
						m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Expected string for field.Text, got %T", field.Text)}
						//time.Sleep(3 * time.Second)
						continue
					}
					m.moduleDebugChan <- DebugMsg{"Found attack narrative: " + audit.Name}
					//time.Sleep(1 * time.Second)
					// Search and replace in the text
					searchString := currentModule.AffectedAmountSearchString
					assetString := currentModule.AffectedAssetsSearchString

					// add this to the module def
					screenshotSearchString := "%KEYS_AGE_SCREENSHOT%"
					replacementScreenshot := ("</p><img class=\"custom-image\" src=\"" + screenshotString + "\" alt=\"" + currentModule.Name + "\"><p>")
					replacementAssets := ""
					for _, asset := range currentModule.AffectedAssets {
						replacementAssets += ("<li><p>" + asset.Name + ": " + asset.ID + "</p></li>\n")
					}

					replacement := strconv.Itoa(len(currentModule.AffectedAssets))

					updatedText := strings.ReplaceAll(text, searchString, replacement)
					updatedAssets := strings.ReplaceAll(updatedText, assetString, replacementAssets)
					updatedScreenshot := strings.ReplaceAll(updatedAssets, screenshotSearchString, replacementScreenshot)

					// Assign the updated text back to the field

					// section.CustomFields[i].Text = updatedAssets
					section.CustomFields[i].Text = updatedScreenshot

					// Update the audit with the new field
					url := fmt.Sprintf("/api/audits/%s/sections/%s", audit.ID, section.ID)
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Updating attack narrative at %s", url)}
					//time.Sleep(1 * time.Second)
					bodyReader, err := util.MarshalStuff(section)
					if err != nil {
						m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error marshalling section: %s", err.Error())}
						//time.Sleep(3 * time.Second)
						continue
					}
					m.moduleDebugChan <- DebugMsg{Message: ("Pre suppress stdout: " + audit.Name)}
					//time.Sleep(1 * time.Second)

					err = nil
					suppressStdout(func() {
						body, err := pwndocApi.PutResponseBody(url, bodyReader)
						if err != nil {
							m.moduleDebugChan <- DebugMsg{fmt.Sprintf("error uploading: %s", err.Error())}
							//time.Sleep(3 * time.Second)

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
					//time.Sleep(6 * time.Second)
					// if err != nil {
					// 	m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error updating attack narrative: %s", err.Error())}
					// 	//time.Sleep(3 * time.Second)
					// 	continue
					// }
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Updated attack narrative: %s", section.Name)}
				}
			}
		}
	}

	// Progress and completion notification
	m.moduleProgressChan <- ModuleProgressMsg{ModuleName: "Attack Narrative", Checked: currentModule.Checked + 1, Total: 10}
	//time.Sleep(1 * time.Second)
	m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: "Attack Narrative"}
}

func createNewFinding(pwndocApi pwndoc.API, m *model, audit pwndoc.APIAudit, currentModule Module, screenshotString string) {
	// Create a new finding for the attack narrative
	m.moduleDebugChan <- DebugMsg{"Creating new finding for " + audit.Name}
	time.Sleep(1 * time.Second)

	retrievedVulnInformation, err := pwndocApi.ExportVulnerabilities()
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error creating file: %s", err.Error())}
		time.Sleep(3 * time.Second)

	}
	//should be a configvar
	if currentModule.Name == "Access Key Age/Last Used" {
		m.moduleDebugChan <- DebugMsg{"Found Access Key Age/Last Used"}
		time.Sleep(1 * time.Second)
	}
	findingTemplateTitle := "Access Keys Older than 90 Days"

	// searchString := currentModule.AffectedAmountSearchString
	// assetString := currentModule.AffectedAssetsSearchString
	searchString := currentModule.AffectedAmountSearchString
	assetString := currentModule.AffectedAssetsSearchString

	replacementAssets := ""
	for _, asset := range currentModule.AffectedAssets {
		replacementAssets += ("<li><p>" + asset.Name + ": " + asset.ID + "</p></li>\n")
	}

	replacement := strconv.Itoa(len(currentModule.AffectedAssets))

	// Assign the updated text back to the field
	// it may be better to first add the finding, then do the replacement on the whole audit?
	//create the finding
	for _, vuln := range retrievedVulnInformation.Data {
		for _, detail := range vuln.Details {

			if detail.Title == findingTemplateTitle {

				detail.Description = strings.ReplaceAll(detail.Description, searchString, replacement)
				detail.Description = strings.ReplaceAll(detail.Description, assetString, replacementAssets)
				detail.Observation = strings.ReplaceAll(detail.Observation, searchString, replacement)
				detail.Observation = strings.ReplaceAll(detail.Observation, assetString, replacementAssets)

				//should move the instructions to a var in the module def
				proofsString := "<ul><li><p>Examine this cat:</p></li></ul><img class=\"custom-image\" src=\"" + screenshotString + "\" alt=\"cat.jpg\">"

				newFinding := pwndoc.APIFindingDetails{
					Title:       detail.Title,
					VulnType:    detail.VulnType,
					Description: detail.Description,
					Observation: detail.Observation,
					Remediation: detail.Remediation,
					References:  detail.References,
					CVSSv3:      vuln.CVSSv3,
					Category:    vuln.Category,
					Scope:       replacementAssets,
					Poc:         proofsString,
					//Upload screenshot before this
					//Poc:         proofsDerections + uploadscreenshot(pwndocApi, m, audit, currentModule),
				}
				bodyReader, err := util.MarshalStuff(newFinding)
				if err != nil {
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("error marshaling vuln: %s", err.Error())}
					time.Sleep(3 * time.Second)

				}

				body, err := pwndocApi.PostResponseBody("/api/audits/"+audit.ID+"/findings", bodyReader)
				if err != nil {
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("error uploading: %s", err.Error())}

				}
				f, err := os.Create("response2.txt")
				if err != nil {
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error creating file: %s", err.Error())}
					time.Sleep(3 * time.Second)
				}
				defer f.Close()
				_, err = f.Write(body)
				if err != nil {
					m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error writing to file: %s", err.Error())}
					time.Sleep(3 * time.Second)
				}

				// retrievedAuditInformation, err := pwndocApi.GetAudit(audit.ID)
				// if err != nil {
				// 	m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Unable to retrieve audit information: %s", err.Error())}
				// 	time.Sleep(3 * time.Second)
				// }
				// for _, finding := range retrievedAuditInformation.Data.Findings {
				// 	if finding.Title == detail.Title {
				// 		updatedFinding := pwndoc.APIFindingDetails{
				// 			Identifier: finding.Identifier,
				// 			Title:      finding.Title,
				// 			VulnType:   finding.VulnType,

			}
		}
	}

}

func takeUploadScreenshot(pwndocApi pwndoc.API, m *model, audit pwndoc.APIAudit, currentModule Module) string {

	//replace this with a configvar
	//wait until the file exists
	// time.Sleep(1 * time.Second)
	cmd := exec.Command("freeze", "/tmp/accesskeyage.csv", "-o", "/tmp/accesskeyage.png", "--window", "--language", "json", "--border.radius", "8") //"--shadow.blur", "20", "--shadow.x", "10", "--shadow.y", "10")

	// Capture the output of the command
	output, err := cmd.Output()
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error executing command: %s", err.Error())}
		time.Sleep(1 * time.Second) // Sleep for 5 seconds before returning
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("freeze command output: %s", output)}
		time.Sleep(1 * time.Second)
		return ""
	}
	m.moduleDebugChan <- DebugMsg{fmt.Sprintf("freeze command output: %s", output)}
	//time.Sleep(5 * time.Second)

	if currentModule.Name == "Access Key Age/Last Used" {
		m.moduleDebugChan <- DebugMsg{"Found Access Key Age/Last Used"}
	}
	// imageData, err := ioutil.ReadFile("/tmp/accesskeyage.png")
	imageData, err := os.ReadFile("/tmp/accesskeyage.png")
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error reading screenshot file: %s", err.Error())}
		time.Sleep(3 * time.Second)
		return ""
	}

	base64Image := base64.StdEncoding.EncodeToString(imageData)
	//marshal the screenshot, json shoud be in the form of {"value": "data:image/png;base64,base64string", name: "accesskeyage.png", auditID: audit.ID}
	screenshot := APIScreenshotUpload{
		Value:   "data:image/png;base64," + base64Image,
		Name:    "accesskeyage.png",
		AuditID: audit.ID,
	}
	bodyReader, err := util.MarshalStuff(screenshot)
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("error marshaling screenshot: %s", err.Error())}
		time.Sleep(3 * time.Second)

	}
	body, err := pwndocApi.PostResponseBody("/api/images", bodyReader)
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("error uploading: %s", err.Error())}

	}
	f, err := os.Create("response3.txt")
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error creating file: %s", err.Error())}
		time.Sleep(3 * time.Second)
	}
	defer f.Close()
	_, err = f.Write(body)
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error writing to file: %s", err.Error())}
		time.Sleep(3 * time.Second)
	}

	var uploadResponse APIScreenshotUploadResponse
	err = json.Unmarshal(body, &uploadResponse)
	if err != nil {
		m.moduleDebugChan <- DebugMsg{fmt.Sprintf("Error unmarshaling json: %s", err.Error())}
		time.Sleep(3 * time.Second)
	}
	return uploadResponse.Datas.ID

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

	// use the captured output if needed
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
