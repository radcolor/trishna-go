//go:build !darwin

package platform

func gpuName() string {
	return "N/A"
}

func hostModel() string {
	return ""
}
