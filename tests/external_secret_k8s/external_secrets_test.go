// Copyright DataStax, Inc.
// Please see the included license file for details.

package external_secrets_k8s

import (
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/k8ssandra/cass-operator/tests/kustomize"
	ginkgo_util "github.com/k8ssandra/cass-operator/tests/util/ginkgo"
	"github.com/k8ssandra/cass-operator/tests/util/kubectl"
)

var (
	testName      = "External secrets test with k8s secrets"
	namespace     = "test-external-secrets-k8s"
	dcName        = "dc2"
	dcYaml        = "../testdata/default-single-rack-single-node-dc-external-k8s-secret.yaml"
	secretYaml    = "../testdata/bob-secret.yaml"
	bobbyuserName = "bob"
	bobbyuserPass = "bobber"
	joeSecretYaml = "../testdata/joe-secret.yaml"
	joeUsername   = "joe"
	joePasss      = "password123"
	ns            = ginkgo_util.NewWrapper(testName, namespace)
)

func TestLifecycle(t *testing.T) {
	AfterSuite(func() {
		logPath := fmt.Sprintf("%s/aftersuite", ns.LogDir)
		err := kubectl.DumpAllLogs(logPath).ExecV()
		if err != nil {
			t.Logf("Failed to dump all the logs: %v", err)
		}

		fmt.Printf("\n\tPost-run logs dumped at: %s\n\n", logPath)
		ns.Terminate()
		err = kustomize.Undeploy(namespace)
		if err != nil {
			t.Logf("Failed to undeploy cass-operator: %v", err)
		}
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, testName)
}

var _ = Describe(testName, func() {
	Context("when in a new cluster where UserInfo is provided", func() {
		Specify("the operator mounts the secret and creates the user", func() {

			By("creating a namespace for the cass-operator")
			err := kubectl.CreateNamespace(namespace).ExecV()
			Expect(err).ToNot(HaveOccurred())

			By("deploy cass-operator with kustomize")
			err = kustomize.Deploy(namespace)
			Expect(err).ToNot(HaveOccurred())
			ns.WaitForOperatorReady()

			step := "create superuser secret"
			k := kubectl.ApplyFiles(secretYaml)
			ns.ExecAndLog(step, k)

			step = "create other superuser secret"
			k = kubectl.ApplyFiles(joeSecretYaml)
			ns.ExecAndLog(step, k)

			step = "creating a DC"
			testFile, err := ginkgo_util.CreateTestFile(dcYaml)
			Expect(err).ToNot(HaveOccurred())

			k = kubectl.ApplyFiles(testFile)
			ns.ExecAndLog(step, k)

			ns.WaitForDatacenterReadyWithTimeouts(dcName, 1200, 1200)

			fmt.Printf("Waiting now..\n")
			time.Sleep(120 * time.Second)

			podNames := ns.GetDatacenterPodNames(dcName)

			step = "check Bobby's credentials work"
			k = kubectl.ExecOnPod(
				podNames[0], "--", "cqlsh",
				"--user", bobbyuserName,
				"--password", bobbyuserPass,
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			ns.ExecAndLog(step, k)

			step = "check Joe's credentials work"
			k = kubectl.ExecOnPod(
				podNames[0], "--", "cqlsh",
				"--user", joeUsername,
				"--password", joePasss,
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			ns.ExecAndLog(step, k)

			step = "check that bad credentials don't work"
			k = kubectl.ExecOnPod(
				podNames[0], "--", "cqlsh",
				"--user", bobbyuserName,
				"--password", "notthepassword",
				"-e", "select * from system_schema.keyspaces;").
				WithFlag("container", "cassandra")
			By(step)
			err = ns.ExecV(k)
			Expect(err).To(HaveOccurred())
		})
	})
})
