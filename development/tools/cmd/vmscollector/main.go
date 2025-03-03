// See https://cloud.google.com/docs/authentication/.
// Use GOOGLE_APPLICATION_CREDENTIALS environment variable to specify
// a service account key file to authenticate to the API.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"regexp"

	"github.com/kyma-project/test-infra/development/tools/pkg/common"
	"github.com/kyma-project/test-infra/development/tools/pkg/vmscollector"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v1"
)

const defaultVMNameExcludeRegexp = "gke-nightly-.*|gke-weekly.*|shoot--kyma-prow.*"
const defaultJobLabelExcludeRegexp = "kyma-gke-nightly|kyma-gke-nightly-.*|kyma-gke-weekly|kyma-gke-weekly-.*"

var (
	project               = flag.String("project", "", "Project ID [Required]")
	dryRun                = flag.Bool("dryRun", true, "Dry Run enabled, nothing is deleted")
	ageInHours            = flag.Int("ageInHours", 3, "VM age in hours. VMs older than: now()-ageInHours are subject to removal.")
	vmNameExcludeRegexp   = flag.String("vmNameRegexp", defaultVMNameExcludeRegexp, "VM name regexp pattern. Matching vms are excluded from removal.")
	jobLabelExcludeRegexp = flag.String("jobLabelRegexp", defaultJobLabelExcludeRegexp, "The regexp pattern for \"job-name\" label defined on the VM object. Matching vms are excluded from removal.")
)

func main() {
	flag.Parse()

	if *project == "" {
		fmt.Fprintln(os.Stderr, "missing -project flag")
		flag.Usage()
		os.Exit(2)
	}

	if *ageInHours < 1 {
		fmt.Fprintln(os.Stderr, "invalid ageInHours value, must be greater than zero")
		flag.Usage()
		os.Exit(2)
	}

	common.ShoutFirst("Running with arguments: project: \"%s\", dryRun: %t, ageInHours: %d, vmNameExcludeRegexp: \"%s\", jobLabelExcludeRegexp: \"%s\"", *project, *dryRun, *ageInHours, *vmNameExcludeRegexp, *jobLabelExcludeRegexp)

	instanceNameExcludeRx := regexp.MustCompile(*vmNameExcludeRegexp)
	jobLabelExcludeRx := regexp.MustCompile(*jobLabelExcludeRegexp)

	context := context.Background()

	connection, err := google.DefaultClient(context, compute.CloudPlatformScope)
	if err != nil {
		log.Fatalf("Could not get authenticated client: %v", err)
	}

	computeSvc, err := compute.New(connection)
	if err != nil {
		log.Fatalf("Could not initialize compute API client: %v", err)
	}

	instancesService := computeSvc.Instances
	instancesAPI := &vmscollector.InstancesAPIWrapper{Context: context, Service: instancesService}
	instanceFilter := vmscollector.DefaultInstanceRemovalPredicate(instanceNameExcludeRx, jobLabelExcludeRx, uint(*ageInHours))

	gc := vmscollector.NewInstancesGarbageCollector(instancesAPI, instanceFilter)

	allSucceeded, err := gc.Run(*project, !(*dryRun))

	if err != nil {
		log.Fatalf("Instances collector error: %v", err)
	}

	if !allSucceeded {
		log.Warn("Some operations failed.")
	}

	common.Shout("Finished")
}
