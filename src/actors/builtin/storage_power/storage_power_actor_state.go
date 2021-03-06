package storage_power

import (
	"sort"

	addr "github.com/filecoin-project/go-address"
	abi "github.com/filecoin-project/specs/actors/abi"
	indices "github.com/filecoin-project/specs/actors/runtime/indices"
	autil "github.com/filecoin-project/specs/actors/util"
	filcrypto "github.com/filecoin-project/specs/algorithms/crypto"
	node_base "github.com/filecoin-project/specs/systems/filecoin_nodes/node_base"
)

func (st *StoragePowerActorState_I) _minerNominalPowerMeetsConsensusMinimum(minerPower abi.StoragePower) bool {

	// if miner is larger than min power requirement, we're set
	if minerPower >= node_base.MIN_MINER_SIZE_STOR {
		return true
	}

	// otherwise, if another miner meets min power requirement, return false
	if st.NumMinersMeetingMinPower() > 0 {
		return false
	}

	// else if none do, check whether in MIN_MINER_SIZE_TARG miners
	if len(st.PowerTable()) <= node_base.MIN_MINER_SIZE_TARG {
		// miner should pass
		return true
	}

	// get size of MIN_MINER_SIZE_TARGth largest miner
	minerSizes := make([]abi.StoragePower, 0, len(st.PowerTable()))
	for _, v := range st.PowerTable() {
		minerSizes = append(minerSizes, v)
	}
	sort.Slice(minerSizes, func(i, j int) bool { return int(i) > int(j) })
	return minerPower >= minerSizes[node_base.MIN_MINER_SIZE_TARG-1]
}

func (st *StoragePowerActorState_I) _slashPledgeCollateral(
	minerAddr addr.Address, slashAmountRequested abi.TokenAmount) abi.TokenAmount {

	Assert(slashAmountRequested >= abi.TokenAmount(0))

	newTable, amountSlashed, ok := autil.BalanceTable_WithSubtractPreservingNonnegative(
		st.EscrowTable(), minerAddr, slashAmountRequested)
	Assert(ok)
	st.Impl().EscrowTable_ = newTable

	TODO()
	// Decide whether we can take any additional action if there is not enough
	// pledge collateral to be slashed.

	return amountSlashed
}

func addrInArray(a addr.Address, list []addr.Address) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// _selectMinersToSurprise implements the PoSt-Surprise sampling algorithm
func (st *StoragePowerActorState_I) _selectMinersToSurprise(challengeCount int, randomness abi.Randomness) []addr.Address {
	// this wont quite work -- a.PowerTable() is a HAMT by actor address, doesn't
	// support enumerating by int index. maybe we need that as an interface too,
	// or something similar to an iterator (or iterator over the keys)
	// or even a seeded random call directly in the HAMT: myhamt.GetRandomElement(seed []byte, idx int) using the ticket as a seed

	ptSize := len(st.PowerTable())
	allMiners := make([]addr.Address, len(st.PowerTable()))
	index := 0

	for address, _ := range st.PowerTable() {
		allMiners[index] = address
		index++
	}

	selectedMiners := make([]addr.Address, 0)
	for chall := 0; chall < challengeCount; chall++ {
		minerIndex := filcrypto.RandomInt(randomness, chall, ptSize)
		potentialChallengee := allMiners[minerIndex]
		// skip dups
		for addrInArray(potentialChallengee, selectedMiners) {
			minerIndex := filcrypto.RandomInt(randomness, chall, ptSize)
			potentialChallengee = allMiners[minerIndex]
		}
		selectedMiners = append(selectedMiners, potentialChallengee)
	}

	return selectedMiners
}

func (st *StoragePowerActorState_I) _getPowerTotalForMiner(minerAddr addr.Address) (
	power abi.StoragePower, ok bool) {

	minerPower, found := st.PowerTable()[minerAddr]
	if !found {
		return abi.StoragePower(0), found
	}

	return minerPower, true
}

func (st *StoragePowerActorState_I) _getCurrPledgeForMiner(minerAddr addr.Address) (currPledge abi.TokenAmount, ok bool) {
	return autil.BalanceTable_GetEntry(st.EscrowTable(), minerAddr)
}

func (st *StoragePowerActorState_I) _addClaimedPowerForSector(minerAddr addr.Address, storageWeightDesc SectorStorageWeightDesc) {
	// Note: The following computation does not use any of the dynamic information from CurrIndices();
	// it depends only on storageWeightDesc. This means that the power of a given storageWeightDesc
	// does not vary over time, so we can avoid continually updating it for each sector every epoch.
	//
	// The function is located in the indices module temporarily, until we find a better place for
	// global parameterization functions.
	sectorPower := indices.ConsensusPowerForStorageWeight(storageWeightDesc)

	currentPower, ok := st.ClaimedPower()[minerAddr]
	Assert(ok)
	st._setClaimedPowerEntryInternal(minerAddr, currentPower+sectorPower)
	st._updatePowerEntriesFromClaimedPower(minerAddr)
}

func (st *StoragePowerActorState_I) _deductClaimedPowerForSectorAssert(minerAddr addr.Address, storageWeightDesc SectorStorageWeightDesc) {
	// Note: The following computation does not use any of the dynamic information from CurrIndices();
	// it depends only on storageWeightDesc. This means that the power of a given storageWeightDesc
	// does not vary over time, so we can avoid continually updating it for each sector every epoch.
	//
	// The function is located in the indices module temporarily, until we find a better place for
	// global parameterization functions.
	sectorPower := indices.ConsensusPowerForStorageWeight(storageWeightDesc)

	currentPower, ok := st.ClaimedPower()[minerAddr]
	Assert(ok)
	st._setClaimedPowerEntryInternal(minerAddr, currentPower-sectorPower)
	st._updatePowerEntriesFromClaimedPower(minerAddr)
}

func (st *StoragePowerActorState_I) _updatePowerEntriesFromClaimedPower(minerAddr addr.Address) {
	claimedPower, ok := st.ClaimedPower()[minerAddr]
	Assert(ok)

	// Compute nominal power: i.e., the power we infer the miner to have (based on the network's
	// PoSt queries), which may not be the same as the claimed power.
	// Currently, the only reason for these to differ is if the miner is in DetectedFault state
	// from a SurprisePoSt challenge.
	nominalPower := claimedPower
	if st.PoStDetectedFaultMiners()[minerAddr] {
		nominalPower = 0
	}
	st._setNominalPowerEntryInternal(minerAddr, nominalPower)

	// Compute actual (consensus) power, i.e., votes in leader election.
	power := nominalPower
	if !st._minerNominalPowerMeetsConsensusMinimum(nominalPower) {
		power = 0
	}

	TODO() // TODO: Decide effect of undercollateralization on (consensus) power.

	st._setPowerEntryInternal(minerAddr, power)
}

func (st *StoragePowerActorState_I) _setClaimedPowerEntryInternal(minerAddr addr.Address, updatedMinerClaimedPower abi.StoragePower) {
	Assert(updatedMinerClaimedPower >= 0)
	st.Impl().ClaimedPower_[minerAddr] = updatedMinerClaimedPower
}

func (st *StoragePowerActorState_I) _setNominalPowerEntryInternal(minerAddr addr.Address, updatedMinerNominalPower abi.StoragePower) {
	Assert(updatedMinerNominalPower >= 0)
	prevMinerNominalPower, ok := st.NominalPower()[minerAddr]
	Assert(ok)
	st.Impl().NominalPower_[minerAddr] = updatedMinerNominalPower

	consensusMinPower := indices.StoragePower_ConsensusMinMinerPower()
	if updatedMinerNominalPower >= consensusMinPower && prevMinerNominalPower < consensusMinPower {
		st.Impl().NumMinersMeetingMinPower_ += 1
	} else if updatedMinerNominalPower < consensusMinPower && prevMinerNominalPower >= consensusMinPower {
		st.Impl().NumMinersMeetingMinPower_ -= 1
	}
}

func (st *StoragePowerActorState_I) _setPowerEntryInternal(minerAddr addr.Address, updatedMinerPower abi.StoragePower) {
	Assert(updatedMinerPower >= 0)
	prevMinerPower, ok := st.PowerTable()[minerAddr]
	Assert(ok)
	st.Impl().PowerTable_[minerAddr] = updatedMinerPower
	st.Impl().TotalNetworkPower_ += (updatedMinerPower - prevMinerPower)
}

func (st *StoragePowerActorState_I) _getPledgeSlashForConsensusFault(currPledge abi.TokenAmount, faultType ConsensusFaultType) abi.TokenAmount {
	// default is to slash all pledge collateral for all consensus fault
	TODO()
	switch faultType {
	case DoubleForkMiningFault:
		return currPledge
	case ParentGrindingFault:
		return currPledge
	case TimeOffsetMiningFault:
		return currPledge
	default:
		panic("Unsupported case for pledge collateral consensus fault slashing")
	}
}

func _getConsensusFaultSlasherReward(elapsedEpoch abi.ChainEpoch, collateralToSlash abi.TokenAmount) abi.TokenAmount {
	TODO()
	// BigInt Operation
	// var growthRate = node_base.SLASHER_SHARE_GROWTH_RATE_NUM / node_base.SLASHER_SHARE_GROWTH_RATE_DENOM
	// var multiplier = growthRate^elapsedEpoch
	// var slasherProportion = min(INITIAL_SLASHER_SHARE * multiplier, 1.0)
	// return collateralToSlash * slasherProportion
	return abi.TokenAmount(0)
}

func PowerTableHAMT_Empty() PowerTableHAMT {
	IMPL_FINISH()
	panic("")
}

func MinerEventsHAMT_Empty() MinerEventsHAMT {
	IMPL_FINISH()
	panic("")
}
