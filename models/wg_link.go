package models

import (
	"github.com/Sakurame1/frp-manager/pb"
	"gorm.io/gorm"
)

// WireGuardLink 鎻忚堪鍚屼竴 Network 涓嬩袱涓?WireGuard 鑺傜偣涔嬮棿鐨勬湁鍚戦摼璺笌鍏舵寚鏍囥€?
// 璇箟锛氫粠 FromWireGuardID 鎸囧悜 ToWireGuardID 鐨勪紶杈撹矾寰勶紝
// UpBandwidthMbps 琛ㄧず浠?From -> To 鐨勫彲鐢ㄤ笂琛屽甫瀹斤紱LatencyMs 涓哄崟鍚戞椂寤躲€?
// 濡傞渶鍙屽悜閾捐矾锛岃鍒涘缓涓ゆ潯瀵瑰悜璁板綍銆?
type WireGuardLink struct {
	gorm.Model
	*WireGuardLinkEntity

	FromWireGuard *WireGuard `json:"from_wireguard,omitempty" gorm:"foreignKey:FromWireGuardID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ToWireGuard   *WireGuard `json:"to_wireguard,omitempty" gorm:"foreignKey:ToWireGuardID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ToEndpoint    *Endpoint  `json:"to_endpoint,omitempty" gorm:"foreignKey:ToEndpointID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

type WireGuardLinkEntity struct {
	// 澶氱鎴?
	UserId   uint32 `gorm:"index"`
	TenantId uint32 `gorm:"index"`

	// 褰掑睘缃戠粶
	NetworkID uint `gorm:"index"`

	// 鏈夊悜杈逛袱绔?
	FromWireGuardID uint `gorm:"index"`
	ToWireGuardID   uint `gorm:"index"`

	ToEndpointID uint `gorm:"index"`

	// 閾捐矾鎸囨爣
	UpBandwidthMbps   uint32
	DownBandwidthMbps uint32
	LatencyMs         uint32

	// 鐘舵€?
	Active bool `gorm:"index"`
}

func (*WireGuardLink) TableName() string {
	return "wireguard_links"
}

func (w *WireGuardLink) FromPB(pbData *pb.WireGuardLink) {
	w.Model = gorm.Model{}
	w.WireGuardLinkEntity = &WireGuardLinkEntity{}

	w.Model.ID = uint(pbData.GetId())
	w.FromWireGuardID = uint(pbData.GetFromWireguardId())
	w.ToWireGuardID = uint(pbData.GetToWireguardId())
	w.UpBandwidthMbps = pbData.GetUpBandwidthMbps()
	w.DownBandwidthMbps = pbData.GetDownBandwidthMbps()
	w.LatencyMs = pbData.GetLatencyMs()
	w.Active = pbData.GetActive()
	w.ToEndpointID = uint(pbData.GetToEndpoint().GetId())
}

func (w *WireGuardLink) ToPB() *pb.WireGuardLink {
	return &pb.WireGuardLink{
		Id:                uint32(w.ID),
		FromWireguardId:   uint32(w.FromWireGuardID),
		ToWireguardId:     uint32(w.ToWireGuardID),
		UpBandwidthMbps:   w.UpBandwidthMbps,
		DownBandwidthMbps: w.DownBandwidthMbps,
		LatencyMs:         w.LatencyMs,
		Active:            w.Active,
		ToEndpoint:        w.ToEndpoint.ToPB(),
	}
}
