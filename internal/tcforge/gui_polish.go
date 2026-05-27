package tcforge

const (
	GUIStatusToneNeutral = "neutral"
	GUIStatusToneSuccess = "success"
	GUIStatusToneWarning = "warning"
	GUIStatusToneError   = "error"
	GUIStatusToneActive  = "active"
)

func GUIStatusTone(status string) string {
	switch status {
	case GUIStatusFixed:
		return GUIStatusToneSuccess
	case GUIStatusNeedsAttention, GUIStatusAlreadyHasTimecode:
		return GUIStatusToneWarning
	case GUIStatusFailed, GUIStatusNoAudioLTCFound:
		return GUIStatusToneError
	case GUIStatusScanning, GUIStatusProcessing:
		return GUIStatusToneActive
	default:
		return GUIStatusToneNeutral
	}
}
