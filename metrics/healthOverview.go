package metrics

import (
	"fmt"
	"sort"

	"github.com/owncast/owncast/core"
	"github.com/owncast/owncast/core/data"
	"github.com/owncast/owncast/models"
	"github.com/owncast/owncast/utils"
)

const (
	healthyPercentageValue   = 75
	maxCPUUsage              = 90
	minClientCountForDetails = 3
)

// GetStreamHealthOverview will return the stream health overview.
func GetStreamHealthOverview() *models.StreamHealthOverview {
	return metrics.streamHealthOverview
}

func generateStreamHealthOverview() {
	overview := &models.StreamHealthOverview{
		Healthy:           true,
		HealthyPercentage: 100,
		Message:           "",
	}

	if cpuUseOverview := cpuUsageHealthOverview(); cpuUseOverview != nil {
		overview = cpuUseOverview
	} else if networkSpeedOverview := networkSpeedHealthOverview(); networkSpeedOverview != nil {
		overview = networkSpeedOverview
	} else if errorCountOverview := errorCountHealthOverview(); errorCountOverview != nil {
		overview = errorCountOverview
	}

	// Determine what percentage of total players are represented in our overview.
	totalPlayerCount := len(core.GetActiveViewers())
	if totalPlayerCount > 0 && len(windowedBandwidths) > 0 {
		representation := utils.IntPercentage(len(windowedBandwidths), totalPlayerCount)
		overview.Representation = representation
	}
	metrics.streamHealthOverview = overview
}

func networkSpeedHealthOverview() *models.StreamHealthOverview {
	type singleVariant struct {
		isVideoPassthrough bool
		bitrate            int
	}

	outputVariants := data.GetStreamOutputVariants()

	streamSortVariants := make([]singleVariant, len(outputVariants))
	for i, variant := range outputVariants {
		variantSort := singleVariant{
			bitrate:            variant.VideoBitrate,
			isVideoPassthrough: variant.IsVideoPassthrough,
		}
		streamSortVariants[i] = variantSort
	}

	sort.Slice(streamSortVariants, func(i, j int) bool {
		if streamSortVariants[i].isVideoPassthrough && !streamSortVariants[j].isVideoPassthrough {
			return true
		}

		if !streamSortVariants[i].isVideoPassthrough && streamSortVariants[j].isVideoPassthrough {
			return false
		}

		return streamSortVariants[i].bitrate > streamSortVariants[j].bitrate
	})

	lowestSupportedBitrate := float64(streamSortVariants[0].bitrate)
	totalNumberOfClients := len(windowedBandwidths)

	if totalNumberOfClients == 0 {
		return nil
	}

	// Determine healthy status based on bandwidth speeds of clients.
	unhealthyClientCount := 0

	for _, speed := range windowedBandwidths {
		if int(speed) < int(lowestSupportedBitrate*1.1) {
			unhealthyClientCount++
		}
	}

	if unhealthyClientCount == 0 {
		return nil
	}

	percentUnhealthy := 100 - ((float64(unhealthyClientCount) / float64(totalNumberOfClients)) * 100)
	healthyPercentage := int(percentUnhealthy)

	return &models.StreamHealthOverview{
		Healthy:           healthyPercentage > healthyPercentageValue,
		Message:           fmt.Sprintf("%d of %d clients (%d%%) are consuming video slower than, or too close to your bitrate of %d kbps.", unhealthyClientCount, totalNumberOfClients, int((float64(unhealthyClientCount)/float64(totalNumberOfClients))*100), int(lowestSupportedBitrate)),
		HealthyPercentage: healthyPercentage,
	}
}

func cpuUsageHealthOverview() *models.StreamHealthOverview {
	if len(metrics.CPUUtilizations) < 2 {
		return nil
	}

	totalNumberOfClients := len(windowedBandwidths)

	recentCPUUses := metrics.CPUUtilizations[len(metrics.CPUUtilizations)-2:]
	values := make([]float64, len(recentCPUUses))
	for i, val := range recentCPUUses {
		values[i] = val.Value
	}
	recentCPUUse := utils.Avg(values)
	if recentCPUUse < maxCPUUsage {
		return nil
	}

	clientsWithErrors := 0
	for _, errors := range windowedErrorCounts {
		if errors > 0 {
			clientsWithErrors++
		}
	}

	healthyPercentage := int(100 - ((float64(clientsWithErrors) / float64(totalNumberOfClients)) * 100))

	return &models.StreamHealthOverview{
		Healthy:           false,
		Message:           fmt.Sprintf("The CPU usage on your server is over %d%%. This may cause video to be provided slower than necessary, causing buffering for your viewers. Consider increasing the resources available or reducing the number of output variants you made available.", maxCPUUsage),
		HealthyPercentage: healthyPercentage,
	}
}

func errorCountHealthOverview() *models.StreamHealthOverview {
	totalNumberOfClients := len(windowedBandwidths)
	if totalNumberOfClients == 0 {
		return nil
	}

	clientsWithErrors := 0
	for _, errors := range windowedErrorCounts {
		if errors > 0 {
			clientsWithErrors++
		}
	}

	if clientsWithErrors == 0 {
		return nil
	}

	// Only return these detailed values and messages if we feel we have enough
	// clients to be able to make a reasonable assessment. This is an arbitrary
	// number but 1 out of 1 isn't helpful.
	message := ""
	healthyPercentage := 0

	if totalNumberOfClients >= minClientCountForDetails {
		healthyPercentage := utils.IntPercentage(clientsWithErrors, totalNumberOfClients)
		message = fmt.Sprintf("%d of %d clients (%d%%) may be experiencing some issues.", clientsWithErrors, totalNumberOfClients, healthyPercentage)

		isUsingPassthrough := false
		outputVariants := data.GetStreamOutputVariants()
		for _, variant := range outputVariants {
			if variant.IsVideoPassthrough {
				isUsingPassthrough = true
			}
		}

		if isUsingPassthrough {
			message = fmt.Sprintf("%d of %d clients (%d%%) are experiencing errors. You're currently using a video passthrough output, often known for causing playback issues for people. It is suggested you turn it off.", clientsWithErrors, totalNumberOfClients, healthyPercentage)
		}
	}

	return &models.StreamHealthOverview{
		Healthy:           healthyPercentage > healthyPercentageValue,
		Message:           message,
		HealthyPercentage: healthyPercentage,
	}
}
