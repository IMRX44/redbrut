package classifier

// AuthResult is the outcome of a single RDP NLA authentication attempt.
type AuthResult int

const (
	ResultSuccess       AuthResult = iota // Server accepted credentials
	ResultInvalid                         // Explicit LOGON_FAILURE — definitely wrong
	ResultLocked                          // Account locked out — stop this user on this IP
	ResultExpired                         // Password expired — credentials are valid
	ResultNetworkError                    // TCP/TLS problem — retry
	ResultProtocolError                   // Unexpected RDP/CredSSP frame — retry
	ResultUnknown                         // Unrecognized status — retry, never mark invalid
)

func (r AuthResult) String() string {
	switch r {
	case ResultSuccess:
		return "SUCCESS"
	case ResultInvalid:
		return "INVALID"
	case ResultLocked:
		return "LOCKED"
	case ResultExpired:
		return "EXPIRED"
	case ResultNetworkError:
		return "NET_ERR"
	case ResultProtocolError:
		return "PROTO_ERR"
	default:
		return "UNKNOWN"
	}
}

// ShouldRetry returns true for transient errors that should be retried.
func (r AuthResult) ShouldRetry() bool {
	return r == ResultNetworkError || r == ResultProtocolError || r == ResultUnknown
}

// IsSuccess returns true if credentials were accepted (including expired passwords).
func (r AuthResult) IsSuccess() bool {
	return r == ResultSuccess || r == ResultExpired
}

// NTSTATUS codes returned by Windows via CredSSP/NTLM.
const (
	NTStatusSuccess          uint32 = 0x00000000
	NTStatusLogonFailure     uint32 = 0xC000006D
	NTStatusAccountLocked    uint32 = 0xC0000234
	NTStatusPasswordExpired  uint32 = 0xC0000071
	NTStatusPasswordMustChange uint32 = 0xC0000224
	NTStatusAccountDisabled  uint32 = 0xC0000072
	NTStatusAccountExpired   uint32 = 0xC0000193
	NTStatusWrongPassword    uint32 = 0xC000006A
	NTStatusNoSuchUser       uint32 = 0xC0000064
)

// FromNTStatus maps a Windows NTSTATUS code to an AuthResult.
// The rule: only return ResultInvalid when the server *explicitly* rejected credentials.
// Everything ambiguous becomes ResultUnknown (→ retry).
func FromNTStatus(status uint32) AuthResult {
	switch status {
	case NTStatusSuccess:
		return ResultSuccess
	case NTStatusLogonFailure, NTStatusWrongPassword, NTStatusNoSuchUser,
		NTStatusAccountDisabled, NTStatusAccountExpired:
		return ResultInvalid
	case NTStatusAccountLocked:
		return ResultLocked
	case NTStatusPasswordExpired, NTStatusPasswordMustChange:
		return ResultExpired
	default:
		return ResultUnknown
	}
}
