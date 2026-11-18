package runtime

import "maps"

func saveStatusDeps() func() {
	origCommandOutput := statusCommandOutput
	origServiceStatus := containerServiceStatus
	origServiceStart := containerServiceStart
	origServiceReady := maps.Clone(containerServiceReady)
	return func() {
		statusCommandOutput = origCommandOutput
		containerServiceStatus = origServiceStatus
		containerServiceStart = origServiceStart
		containerServiceReady = origServiceReady
	}
}
