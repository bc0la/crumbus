package cmd

import (
	"fmt"
	"log"

	//"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
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

type ModuleAffectedAssetMsg struct {
	ModuleName    string
	AffectedAsset AffectedAsset
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

		//time.Sleep(2 * time.Second)
		//return nil
		return tea.Batch(cmds...)
	}
}

func AccesKeyScoutQuery(m *model, currentMod string, reportFiles []string, numReports int) error {

	jsonDataList := openJsonFiles(m, reportFiles)
	//time.Sleep(2 * time.Second)

	//loop through reports
	currentReportNum := 1
	for _, jsonData := range jsonDataList {

		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Running module %s", currentMod)}

		//Query string for the Active Access Key Age > 90 days
		AccessKeyAgeAffectedQueryString := ".services.iam.findings[\"iam-user-no-Active-key-rotation\"].items"

		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Checking report number %d of %d", currentReportNum, numReports)}

		// Gets the locations of the affected assets for the Access Key Age
		findingAssetIDS := jqFindings(m, jsonData, currentMod, AccessKeyAgeAffectedQueryString)

		// transforms each asset ID into a path that can be used to query the json data
		var affectedAssetsKeyIDs []string
		var affectedAssetsKeyUser []string
		checkedAssets := 0
		for _, id := range findingAssetIDS {

			//makes access key id query string from the affected asset
			affectedKeyIDpath := (".services" + transformPath(id) + ".AccessKeyId")
			affectedKeyUserNamepath := (".services" + transformPath(id) + ".UserName")

			m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Transformed Path: %s", affectedKeyIDpath)}
			//time.Sleep(2 * time.Second)
			// Query the json data for the specific assets
			accessKeyID := jqAssets(m, jsonData, currentMod, affectedKeyIDpath)
			accessKeyUserName := jqAssets(m, jsonData, currentMod, affectedKeyUserNamepath)

			//add the info to the slices
			affectedAssetsKeyIDs = append(affectedAssetsKeyIDs, accessKeyID)
			affectedAssetsKeyUser = append(affectedAssetsKeyUser, accessKeyUserName)

			checkedAssets++

			// Send the affected asset to the model
			m.moduleAffectedAssetChan <- ModuleAffectedAssetMsg{ModuleName: currentMod, AffectedAsset: AffectedAsset{ID: accessKeyID, Name: accessKeyUserName}}

			m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: checkedAssets, StatusMessage: "Checking " + accessKeyID}

		}

		// Should update the model in the update function perhaps

		m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: checkedAssets, StatusMessage: fmt.Sprintf("Finished: %v %v", affectedAssetsKeyIDs, affectedAssetsKeyUser)}
		m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("Affected Assets: %v", affectedAssetsKeyIDs)}

		//m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: checkedCount, Total: totalCount}
		// time.Sleep(2 * time.Second)

		currentReportNum++
	}

	m.moduleDoneChan <- ModuleCompleteMsg{ModuleName: currentMod}
	return nil
}

// This function pulls the ids of the affected assets from the json data, and gets a count of the total number of affected assets
func jqFindings(m *model, jsonData interface{}, currentMod string, findingQueryString string) []string {

	// Parse the string to create a gojq query
	findingQuery, err := gojq.Parse(findingQueryString)
	if err != nil {
		m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "countAgeQuery Parse: " + err.Error()}
		time.Sleep(3 * time.Second)

	}

	//this gets us the total value for displaying in our execution view, it's not efficent but just in case I do more processing later
	countIter := findingQuery.Run(jsonData)
	totalCount := 0
	for {
		_, ok := countIter.Next()
		if !ok {
			break
		}
		totalCount++

	}
	m.moduleProgressChan <- ModuleProgressMsg{ModuleName: currentMod, Checked: 0, Total: totalCount, StatusMessage: "Counting"}

	// Perform full query against the scoutsuite finding
	// this may be redundant
	accessKeyAgeQuery, err := gojq.Parse(findingQueryString)
	if err != nil {
		m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery Parse: " + err.Error()}

	}

	findingIter := accessKeyAgeQuery.Run(jsonData)
	// checkedCount := 0
	var results []string
	for {
		v, ok := findingIter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery iter: " + err.Error()}
			return nil
		}
		switch value := v.(type) {
		case string:
			results = append(results, value)
			m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("v: %v", value)}
			//time.Sleep(2 * time.Second)
		case []interface{}:
			for _, elem := range value {
				strElem := fmt.Sprint(elem) // Convert each element to string
				results = append(results, strElem)
				m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("v: %v", strElem)}
			}
		default:
			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery iter: " + "value is not a string" + fmt.Sprintf("v: %v %T", value, value)}
			results = append(results, fmt.Sprint(value))

		}

	}
	return results
}

// This function pulls the value from the id of the affected assets from the finding's affected asset path based on the transformed function
func jqAssets(m *model, jsonData interface{}, currentMod string, findingQueryString string) string {

	// Parse the string to create a gojq query

	accessKeyAgeQuery, err := gojq.Parse(findingQueryString)
	if err != nil {
		m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery Parse: " + err.Error()}

	}
	// Perform full query against the scoutsuite affected asset
	findingIter := accessKeyAgeQuery.Run(jsonData)
	// checkedCount := 0
	var results string
	for {
		v, ok := findingIter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery iter: " + err.Error()}

		}
		switch value := v.(type) {
		case string:
			results = value
			m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("v: %v", value)}
			//time.Sleep(2 * time.Second)
			return results
		case []interface{}:
			for _, elem := range value {
				strElem := fmt.Sprint(elem) // Convert each element to string
				// there should just be one elemet in the slice

				results = strElem

				m.moduleDebugChan <- DebugMsg{Message: fmt.Sprintf("v: %v", strElem)}
				return results
			}

		default:
			m.moduleErrChan <- ModuleErrMsg{ModuleName: currentMod, ErrorMessage: "accessKeyAgeQuery iter: " + "value is not a string"}

		}

	}
	return results
}

func openJsonFiles(m *model, reportFiles []string) []interface{} {
	// var affectedAssets int
	//
	jsonDataList := make([]interface{}, 0, len(reportFiles))
	// i think maybe i should wrap everything in the for loop, or find an easier way to separate multiple reports
	for _, file := range reportFiles {
		m.moduleDebugChan <- DebugMsg{Message: "Processing file: " + file}
		//time.Sleep(2 * time.Second)
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

	return jsonDataList
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

	totalBuckets := 4 // Dummy value for total buckets
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

func affectedAssetListen(affectedAssetChan chan ModuleAffectedAssetMsg) tea.Cmd {
	return func() tea.Msg {
		return ModuleAffectedAssetMsg(<-affectedAssetChan)
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

func transformPath(input string) string {
	parts := strings.Split(input, ".")
	result := ""
	for i, part := range parts {
		if num, err := strconv.Atoi(part); err == nil {
			// If it's a number, convert to bracket notation
			result += fmt.Sprintf("[%d]", num)
		} else {
			// For normal string parts, add with a leading dot if it's not the first element
			if i > 0 {
				result += "."
			}
			result += part
		}
	}
	return "." + result // Ensure the result starts with a dot
}
