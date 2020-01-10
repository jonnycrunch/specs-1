package abi

import (
	"github.com/ipfs/go-cid"
)

// SectorNumber is a numeric identifier for a sector. It is usually relative to a miner.
type SectorNumber int64

// SectorSize indicates one of a set of possible sizes in the network.
// Ideally, SectorSize would be an enum
// type SectorSize enum {
//   1KiB = 1024
//   1MiB = 1048576
//   1GiB = 1073741824
//   1TiB = 1099511627776
//   1PiB = 1125899906842624
// }
type SectorSize int64

// TODO make sure this is globally unique
type SectorID struct {
	Miner  int64 // Actor ID (from address)
	Number SectorNumber
}

// The unit of sector weight (power-epochs)
type SectorWeight int64 // TODO bigint

// The unit of storage power (measured in bytes)
type StoragePower int64 // TODO bigint

type UnsealedSectorCID cid.Cid
type SealedSectorCID cid.Cid

type SectorPreCommitInfo struct {
	SectorNumber
	SealedCID  SealedSectorCID // CommR
	SealEpoch  ChainEpoch
	DealIDs    DealIDs
	Expiration ChainEpoch
}

type SectorProveCommitInfo struct {
	SectorNumber
	Proof            SealProof
	InteractiveEpoch ChainEpoch
	Expiration       ChainEpoch
}

type SealProof struct {
	//<curve, system> {
	Config     SealProofConfig
	ProofBytes []byte
}

type SealProofConfig struct {
	// TODO
}

// TODO: PieceInfo is the name used by proofs for this struct, but there also exists a piece.PieceInfo type, which is different.
// We should probably rename one of them, since this is quite confusing. Or, if we can put `PieceCID` into pieces.PieceInfo, we can just use that.
type PieceInfo struct {
	Size     int64 // Size in nodes. For BLS12-381 (capacity 254 bits), must be >= 16. (16 * 8 = 128)
	PieceCID PieceCID
}

type PieceInfos struct {
	Items []PieceInfo
}


type PartialTicket []byte // 32 bytes

type PoStCandidate struct {
	PartialTicket                            // Optional —  will eventually be omitted for SurprisePoSt verification, needed for now.
	PrivateProof   PrivatePoStCandidateProof // Optional — should be ommitted for verification.
	SectorID       SectorID
	ChallengeIndex int64
}

type PrivatePoStCandidateProof struct{}
