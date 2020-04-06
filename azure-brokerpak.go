package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/azure-storage-file-go/azfile"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/writeameer/aci/helpers"
)

var (
	ctx = context.Background()
)

// Auth Checks creds are provided in the ENV and returns an Azure token and Subscription ID
func Auth() (authorizer autorest.Authorizer, sid string) {
	// Check env for creds and read env
	helpers.FatalError(helpers.CheckEnv())
	sid = os.Getenv("AZURE_SUBSCRIPTION_ID")

	// Authenticate with Azure
	log.Println("Starting azure auth...")
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	helpers.FatalError(err)
	log.Println("After azure auth...")

	return
}

// CreateStorageAccount Creates an Azure storage account
func getStorageAccount(resourceGroupName string, storageAccountName string) (keys *[]storage.AccountKey, err error) {
	// Authenticate with Azure
	fmt.Println("*************")
	authorizer, sid := Auth()
	//fmt.Println(sid)

	client := storage.NewAccountsClient(sid)
	client.Authorizer = authorizer

	result, err := client.ListKeys(ctx, resourceGroupName, storageAccountName, "")
	keys = result.Keys

	fmt.Println("*************")
	return
}

// CreateAzureFileShare Creates an Azure File Share
func CreateAzureFileShare(resourceGroupName string, storageAccountName string, shareName string) (key string, err error) {

	// Create storage account
	keys, err := getStorageAccount(resourceGroupName, storageAccountName)
	storageKey := (*keys)[0]
	key = *storageKey.Value

	cred, err := azfile.NewSharedKeyCredential(storageAccountName, key)
	// if err != nil {
	// 	return nil, err
	// }

	p := azfile.NewPipeline(cred, azfile.PipelineOptions{})
	//fmt.Println(p)
	brokerURL, _ := url.Parse(os.Getenv("DOWNLOAD_URL"))

	srcFileURL := azfile.NewFileURL(*brokerURL, p)

	downloadResponse, err := srcFileURL.Download(ctx, 0, azfile.CountToEnd, false)

	if err != nil {

		return
	}

	//contentLength := downloadResponse.ContentLength() // Used for progress reporting to report the total number of bytes being downloaded.

	// Setup RetryReader options for stream reading retry.
	retryReader := downloadResponse.Body(azfile.RetryReaderOptions{MaxRetryRequests: 3})

	// NewResponseBodyStream wraps the RetryReader with progress reporting; it returns an io.ReadCloser.
	progressReader := pipeline.NewResponseBodyProgress(retryReader,
		func(bytesTransferred int64) {
			//fmt.Printf("Downloaded %d of %d bytes.\n", bytesTransferred, contentLength)
		})
	defer progressReader.Close() // The client must close the response body when finished with it

	file, err := os.Create("azure-services-0.1.0.brokerpak") // Create the file to hold the downloaded file contents.
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	written, err := io.Copy(file, progressReader) // Write to the file by reading from the file (with intelligent retries).
	if err != nil {
		log.Fatal(err)
	}
	_ = written // Avoid compiler's "declared and not used" error

	return
}
