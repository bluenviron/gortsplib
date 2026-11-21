package gortsplib

import (
	"slices"
	"time"

	"github.com/pion/rtcp"
)

// Unassociated sources must remain memory-bounded before their first valid media packet.
const maxPendingSenderReports = 128

type receivedSenderReport struct {
	report     rtcp.SenderReport
	receivedAt time.Time
}

type senderReportBuffer struct {
	entries []receivedSenderReport
}

func (b *senderReportBuffer) add(report *rtcp.SenderReport, receivedAt time.Time) {
	index := slices.IndexFunc(b.entries, func(entry receivedSenderReport) bool {
		return entry.report.SSRC == report.SSRC
	})
	if index < 0 && len(b.entries) == maxPendingSenderReports {
		index = 0
	}
	if index >= 0 {
		b.entries = slices.Delete(b.entries, index, index+1)
	}
	b.entries = append(b.entries, receivedSenderReport{
		report:     rtcp.SenderReport{SSRC: report.SSRC, NTPTime: report.NTPTime, RTPTime: report.RTPTime},
		receivedAt: receivedAt,
	})
}

func (b *senderReportBuffer) take(source uint32) (receivedSenderReport, bool) {
	index := slices.IndexFunc(b.entries, func(entry receivedSenderReport) bool {
		return entry.report.SSRC == source
	})
	if index < 0 {
		return receivedSenderReport{}, false
	}
	entry := b.entries[index]
	b.entries = slices.Delete(b.entries, index, index+1)
	return entry, true
}
