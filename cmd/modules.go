package cmd

import (
	"fmt"
	"log"

	//"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/itchyny/gojq"
)

type ModuleProgressMsg struct {
	ModuleName    string
	Checked       int
	Total         int
	StatusMessage string
}

type ModuleCompleteMsg struct {
	ModuleName string
}

type ModuleErrMsg struct {
	ModuleName   string
	ErrorMessage string
}

type DebugMsg struct {
	Message string
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			panic(err) // or handle error more gracefully
		}
		homeDir := usr.HomeDir
		return filepath.Join(homeDir, path[2:])
	}

	return path
}

// Handle which modules are run
func ModuleRunner(m *model) tea.Cmd {
	return func() tea.Msg {
		var cmds []tea.Cmd
		scoutDir := expandHome(m.ConfigVars.ScoutSuiteReportsDir)
		var reportFiles []string

		m.moduleDebugChan <- DebugMsg{Message: "about to walk: " + scoutDir}
		//time.Sleep(2 * time.Second)
		err := filepath.WalkDir(scoutDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				m.moduleDebugChan <- DebugMsg{Message: "filepathWalk Error " + err.Error()}
				return nil // or return err to stop walking the directory
			}
			if !d.IsDir() && strings.HasPrefix(d.Name(), "scoutsuite_results_") && strings.HasSuffix(d.Name(), ".js") {
				reportFiles = append(reportFiles, path)
			}
			return nil
		})

		if err != nil {
			m.moduleDebugChan <- DebugMsg{Message: err.Error()}
			return nil
		}
		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Found %d reports", len(reportFiles))}
		//time.Sleep(2 * time.Second)

		for _, module := range m.PwndocModules {
			if module.Selected && module.Name == "Access Key Age/Last Used" {
				go AccesKeyScoutQuery(m, module.Name, reportFiles, len(reportFiles))
			}

			if module.Selected && module.Name == "Open S3 Buckets (Authenticated/Anonymous)" {
				//go S3ScoutQuery(m, module.Name, reportFiles, len(reportFiles))
				cmds = append(cmds, OpenS3(module.Name, m.moduleProgressChan, m.moduleDoneChan))
			}
			//m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Module %s selected: %t", module.Name, module.Selected)}
		}

		time.Sleep(2 * time.Second)
		//return nil
		return tea.Batch(cmds...)
	}
}

func AccesKeyScoutQuery(m *model, currentMod string, reportFiles []string, numReports int) error {

	jsonDataList := make([]interface{}, 0, len(reportFiles))

	for _, file := range reportFiles {
		m.moduleDebugChan <- DebugMsg{Message: "Processing file: " + file}
		time.Sleep(2 * time.Second)
		jsonData, err := preprocessFile(file)
		if err != nil {
			m.moduleDebugChan <- DebugMsg{Message: err.Error()}
			continue
		}
		jsonDataList = append(jsonDataList, jsonData)
	}
	m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Finished Processing %d reports", len(reportFiles))}
	for _, jsonData := range jsonDataList {
		if jsonData == nil {
			m.moduleDebugChan <- DebugMsg{Message: "jsonData is nil, skipping"}
			time.Sleep(2 * time.Second)
			continue
		}
	}
	//time.Sleep(2 * time.Second)
	//var cmds []tea.Cmd

	for _, jsonData := range jsonDataList {

		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Running module %s", currentMod)}
		currentReportNum := 1

		time.Sleep(2 * time.Second) // Simulate delay

		AccessKeyAgeAffectedQueryString := ".services.iam.findings[\"iam-user-no-Active-key-rotation\"].items"
		//

		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Checking report number %d of %d", currentReportNum, numReports)}
		// Execute the counting query
		countAgeQuery, err := gojq.Parse(AccessKeyAgeAffectedQueryString)
		if err != nil {
			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "countAgeQuery Parse: " + err.Error()}
			time.Sleep(3 * time.Second)

		}

		countIter := countAgeQuery.Run(jsonData)
		totalCount := 0
		for {
			_, ok := countIter.Next()
			if !ok {
				break
			}
			totalCount++

		}
		m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: 0, Total: totalCount, StatusMessage: "Counting"}
		// Perform full query
		accessKeyAgeQuery, err := gojq.Parse(AccessKeyAgeAffectedQueryString)
		if err != nil {
			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery Parse: " + err.Error()}

		}

		accessKeyIter := accessKeyAgeQuery.Run(jsonData)
		checkedCount := 0
		for {
			v, ok := accessKeyIter.Next()
			if !ok {
				break
			}
			if err, isErr := v.(error); isErr {
				m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery iter: " + err.Error()}
				return nil
			}
			m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: checkedCount, Total: totalCount, StatusMessage: fmt.Sprintf("v: %v", v)}
			time.Sleep(2 * time.Second)
			checkedCount++
		}
		currentReportNum++
	}
	m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: currentMod}
	return nil
}

// func S3ScoutQuery(m *model, currentMod string, reportFiles []string, numReports int) error {
// 	jsonDataList := make([]interface{}, 0, len(reportFiles))

// 	for _, file := range reportFiles {
// 		m.moduleDebugChan <- DebugMsg{Message: "Processing file: " + file}
// 		time.Sleep(2 * time.Second)
// 		jsonData, err := preprocessFile(file)
// 		if err != nil {
// 			m.moduleDebugChan <- DebugMsg{Message: err.Error()}
// 			continue
// 		}
// 		jsonDataList = append(jsonDataList, jsonData)
// 	}
// 	m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Finished Processing %d reports", len(reportFiles))}
// 	for _, jsonData := range jsonDataList {
// 		if jsonData == nil {
// 			m.moduleDebugChan <- DebugMsg{Message: "jsonData is nil, skipping"}
// 			time.Sleep(2 * time.Second)
// 			continue
// 		}
// 	}
// 	//time.Sleep(2 * time.Second)
// 	// var cmds []tea.Cmd

// 	for _, jsonData := range jsonDataList {

// 		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Running module %s", currentMod)}
// 		currentReportNum := 1

// 		time.Sleep(2 * time.Second) // Simulate delay

// 		s3PublicQuery := "."

// 		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Checking report number %d of %d", currentReportNum, numReports)}
// 		// Execute the counting query
// 		countQuery, err := gojq.Parse(s3PublicQuery)
// 		if err != nil {
// 			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "countQuery Parse: " + err.Error()}
// 			time.Sleep(3 * time.Second)

// 		}

// 		countIter := countQuery.Run(jsonData)
// 		totalCount := 0
// 		for {
// 			_, ok := countIter.Next()
// 			if !ok {
// 				break
// 			}
// 			totalCount++
// 			m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: 0, Total: totalCount, StatusMessage: "Counting"}
// 		}

// 		// Perform full query
// 		query, err := gojq.Parse(s3PublicQuery)
// 		if err != nil {
// 			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "publicQuery Parse: " + err.Error()}

// 		}

// 		s3Iter := query.Run(jsonData)
// 		checkedCount := 0
// 		for {
// 			v, ok := s3Iter.Next()
// 			if !ok {
// 				break
// 			}
// 			if err, isErr := v.(error); isErr {
// 				m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "query iter: " + err.Error()}
// 				return nil
// 			}
// 			m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: checkedCount, Total: totalCount, StatusMessage: fmt.Sprintf("v: %v", v)}
// 			checkedCount++
// 		}
// 		currentReportNum++
// 	}
// 	m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: currentMod}
// 	return nil
// }

func OpenS3(moduleName string, moduleProgressChan chan<- ModuleProgressMsg, moduleDoneChan chan<- ModuleCompleteMsg) tea.Cmd {

	totalBuckets := 10 // Dummy value for total buckets
	for i := 1; i <= totalBuckets; i++ {
		time.Sleep(time.Second) // Simulate delay
		//println("Checking bucket", i, "in module", moduleName)
		f, err := os.OpenFile("s3progress.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		if _, err := f.Write([]byte("Sending Progress message\n")); err != nil {
			log.Fatal(err)
		}

		moduleProgressChan <- ModuleProgressMsg{ModuleName: moduleName, Checked: i, Total: totalBuckets}
	}
	moduleDoneChan <- ModuleCompleteMsg{ModuleName: moduleName}
	return nil

}

func checkRegionBuckets(regionName string) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(regionName),
	})
	if err != nil {
		fmt.Println("Error creating session for region", regionName, ":", err)
		return
	}

	svc := s3.New(sess)
	result, err := svc.ListBuckets(nil)
	if err != nil {
		fmt.Println("Error listing buckets in region", regionName, ":", err)
		return
	}

	for _, bucket := range result.Buckets {
		go checkBucketAccess(svc, *bucket.Name)
	}
}

func checkBucketAccess(svc *s3.S3, bucketName string) {
	input := &s3.ListObjectsInput{
		Bucket: aws.String(bucketName),
	}

	_, err := svc.ListObjects(input)
	if err != nil {
		fmt.Println("Bucket not accessible anonymously:", bucketName)
	} else {
		fmt.Println("Bucket accessible anonymously:", bucketName)
	}
}

func moduleProgressListen(moduleProgressChan chan ModuleProgressMsg) tea.Cmd {
	return func() tea.Msg {
		// f, err := os.OpenFile("s3progress.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// defer f.Close()

		// if _, err := f.Write([]byte("Listening for progress\n")); err != nil {
		// 	log.Fatal(err)
		// }

		return ModuleProgressMsg(<-moduleProgressChan)
	}
}

func moduleDoneListen(moduleDoneChan chan ModuleCompleteMsg) tea.Cmd {
	return func() tea.Msg {
		return ModuleCompleteMsg(<-moduleDoneChan)
	}
}

func moduleErrListen(moduleErrChan chan ModuleErrMsg) tea.Cmd {
	return func() tea.Msg {
		return ModuleErrMsg(<-moduleErrChan)
	}
}

func debugListen(debugChan chan DebugMsg) tea.Cmd {
	return func() tea.Msg {
		return DebugMsg(<-debugChan)
	}
}

func preprocessFile(filePath string) (interface{}, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Convert data to string and remove the first line (assuming JSON starts on the second line)
	content := string(data)
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) < 2 {
		return nil, fmt.Errorf("not enough data in file: %s", filePath)
	}

	var jsonObj interface{}
	if err := json.Unmarshal([]byte(lines[1]), &jsonObj); err != nil {
		return nil, err
	}
	return jsonObj, nil
}
