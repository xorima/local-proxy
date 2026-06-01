package model

type NTLMMode int

const (
	NTLMModeV1 NTLMMode = iota
	NTLMModeV2
	NTLMModeSession
)

type NTLMState int

const (
	NTLMInit NTLMState = iota
	NTLMNegotiate
	NTLMChallengeReceived
	NTLMAuthenticate
)

type ChallengeInfo struct {
	Raw        []byte
	Nonce      []byte
	Flags      uint32
	TargetName string
	TargetInfo []byte
}
