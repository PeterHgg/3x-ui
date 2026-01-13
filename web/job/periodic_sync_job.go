package job

import (
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// PeriodicSyncJob periodically syncs all slave inbounds from their source inbounds.
// This ensures that any missed real-time syncs are automatically recovered.
type PeriodicSyncJob struct {
	inboundService service.InboundService
}

// NewPeriodicSyncJob creates a new periodic sync job.
func NewPeriodicSyncJob() *PeriodicSyncJob {
	return &PeriodicSyncJob{}
}

// Run syncs all slave inbounds (those with SyncSourceId > 0) from their source.
func (j *PeriodicSyncJob) Run() {
	db := database.GetDB()
	var slaveInbounds []*model.Inbound
	err := db.Where("sync_source_id > 0").Find(&slaveInbounds).Error
	if err != nil {
		logger.Warning("PeriodicSyncJob: Failed to get slave inbounds:", err)
		return
	}

	if len(slaveInbounds) == 0 {
		return
	}

	logger.Debugf("PeriodicSyncJob: Checking %d slave inbounds for sync", len(slaveInbounds))

	syncedCount := 0
	for _, slave := range slaveInbounds {
		// Get source inbound to compare client count
		sourceInbound, err := j.inboundService.GetInbound(slave.SyncSourceId)
		if err != nil {
			logger.Warning("PeriodicSyncJob: Failed to get source inbound", slave.SyncSourceId, ":", err)
			continue
		}

		// Get client counts
		sourceClients, _ := j.inboundService.GetClients(sourceInbound)
		slaveClients, _ := j.inboundService.GetClients(slave)

		// Only sync if counts differ (indicates missed sync)
		if len(sourceClients) != len(slaveClients) {
			logger.Infof("PeriodicSyncJob: Syncing inbound %d (%s) - source has %d clients, slave has %d",
				slave.Id, slave.Remark, len(sourceClients), len(slaveClients))

			_, err := j.inboundService.PerformFullSync(slave.Id)
			if err != nil {
				logger.Warning("PeriodicSyncJob: Failed to sync inbound", slave.Id, ":", err)
			} else {
				syncedCount++
			}
		}
	}

	if syncedCount > 0 {
		logger.Infof("PeriodicSyncJob: Synced %d slave inbounds", syncedCount)
	}
}
