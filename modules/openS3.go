package modules

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	tea "github.com/charmbracelet/bubbletea"
)

type errMsg struct {
	Err error
}

type S3ProgressMsg struct {
	ModuleName string
	Checked    int
	Total      int
}

type S3CompleteMsg struct {
	ModuleName string
}

func OpenS3(moduleName string, s3ProgressChan chan<- S3ProgressMsg, s3DoneChan chan<- S3CompleteMsg) tea.Cmd {
	return func() tea.Msg {
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

			s3ProgressChan <- S3ProgressMsg{ModuleName: moduleName, Checked: i, Total: totalBuckets}
		}
		s3DoneChan <- S3CompleteMsg{ModuleName: moduleName}
		return nil
	}
}

// openS3 checks S3 buckets in all regions for public access
// func OpenS3(moduleName string, progressCh chan<- S3ProgressMsg, doneCh chan<- S3CompleteMsg) tea.Cmd {

// 	return func() tea.Msg {
// 		// First, create a session to list all regions
// 		sess, err := session.NewSession(&aws.Config{
// 			Region: aws.String("us-east-1"), // Can list regions from any global region endpoint
// 		})
// 		if err != nil {
// 			return errMsg{Err: err}
// 		}

// 		// List all available regions
// 		ec2Svc := ec2.New(sess)
// 		regions, err := ec2Svc.DescribeRegions(&ec2.DescribeRegionsInput{})
// 		if err != nil {
// 			return errMsg{Err: err}
// 		}

// 		// Iterate over each region and check S3 buckets
// 		for _, region := range regions.Regions {
// 			regionName := *region.RegionName
// 			go checkRegionBuckets(regionName)
// 		}

// 		return nil // Or some appropriate message
// 	}
// }

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
