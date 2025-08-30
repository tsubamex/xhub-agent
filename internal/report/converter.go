package report

import (
	"xhub-agent/internal/monitor"
	pb "xhub-agent/proto/reportpb"
)

// ConvertToProto converts monitor.ServerStatusData to protobuf format
func ConvertToProto(data *monitor.ServerStatusData) *pb.ServerStatusData {
	if data == nil {
		return nil
	}

	return &pb.ServerStatusData{
		Cpu:         data.CPU,
		CpuCores:    int32(data.CPUCores),
		LogicalPro:  int32(data.LogicalPro),
		CpuSpeedMhz: data.CPUSpeedMhz,
		Memory: &pb.MemoryInfo{
			Current: data.Memory.Current,
			Total:   data.Memory.Total,
		},
		Swap: &pb.SwapInfo{
			Current: data.Swap.Current,
			Total:   data.Swap.Total,
		},
		Disk: &pb.DiskInfo{
			Current: data.Disk.Current,
			Total:   data.Disk.Total,
		},
		Uptime:   int32(data.Uptime),
		Loads:    data.Loads,
		TcpCount: int32(data.TCPCount),
		UdpCount: int32(data.UDPCount),
		NetIo: &pb.NetIOInfo{
			Up:   data.NetIO.Up,
			Down: data.NetIO.Down,
		},
		NetTraffic: &pb.NetTraffic{
			Sent: data.NetTraffic.Sent,
			Recv: data.NetTraffic.Recv,
		},
		PublicIp: &pb.PublicIPInfo{
			Ipv4: data.PublicIP.IPv4,
			Ipv6: data.PublicIP.IPv6,
		},
		Xray: &pb.XrayInfo{
			State:    data.Xray.State,
			ErrorMsg: data.Xray.ErrorMsg,
			Version:  data.Xray.Version,
		},
		AppStats: &pb.AppStats{
			Threads: int32(data.AppStats.Threads),
			Memory:  data.AppStats.Memory,
			Uptime:  int32(data.AppStats.Uptime),
		},
	}
}
