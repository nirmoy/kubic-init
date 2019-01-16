package clusterhealthtest

import (
	"fmt"
	"os"

	. "github.com/kubic-project/kubic-init/tests/lib"
	. "github.com/onsi/ginkgo"
)

var host string

// use init function for reading IPs of cluster
func init() {
	host = os.Getenv("SEEDER")
	if host == "" {
		panic("SEEDER IP not set")
	}
}

var _ = Describe("Check Master health", func() {
	It("kubic-init systemd service is up and running", func() {
		cmd := "systemctl is-active --quiet kubic-init.service"
		output, err := RunCmd(host, cmd)
		if err != nil {
			fmt.Printf("[ERROR]: kubic-init.service failed")
			panic(err)
		}
		fmt.Println(string(output))
	})

})
