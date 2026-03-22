package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	defaultGuideURL   = "https://www.kdocs.cn/l/cejYQ4grndXr?source=wiki"
	defaultUsageURL   = "https://ai.dsx-family.site/subscriptions"
	defaultNoticeText = "这是请求地址和密钥：请查看教程，创建完密钥后，查看使用密钥就可以看到"
	defaultDisclaimer = "卡密（API Key）为虚拟商品，一旦发出，无法退换。感谢您的理解！"
)

type WeComHandler struct {
	cfg           *config.Config
	redeemService *service.RedeemService
	groupService  *service.GroupService
}

func NewWeComHandler(cfg *config.Config, redeemService *service.RedeemService, groupService *service.GroupService) *WeComHandler {
	return &WeComHandler{
		cfg:           cfg,
		redeemService: redeemService,
		groupService:  groupService,
	}
}

type GenerateChannelContentRequest struct {
	GroupName    string  `json:"group_name"`
	ValidityDays int     `json:"validity_days"`
	RedeemValue  float64 `json:"redeem_value"`
	Source       string  `json:"source"`
	SourceUser   string  `json:"source_user"`
}

type GenerateChannelContentResponse struct {
	GroupID        int64  `json:"group_id"`
	GroupName      string `json:"group_name"`
	InvitationCode string `json:"invitation_code"`
	RedeemCode     string `json:"redeem_code"`
	Content        string `json:"content"`
}

// GenerateChannelContent generates invitation/redeem code content for any channel.
// POST /api/v1/channel/content/generate
func (h *WeComHandler) GenerateChannelContent(c *gin.Context) {
	reqLog := requestLogger(c, "handler.channel_content.generate")
	var req GenerateChannelContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		reqLog.Warn("channel_content.request_validation_failed", zap.String("reason", "invalid_request_body"), zap.Error(err))
		response.BadRequest(c, "Invalid request body")
		return
	}
	req.GroupName = strings.TrimSpace(req.GroupName)
	req.Source = strings.TrimSpace(req.Source)
	req.SourceUser = strings.TrimSpace(req.SourceUser)
	if req.Source == "" {
		req.Source = "channel"
	}
	noteUser := req.SourceUser
	if noteUser == "" {
		noteUser = "unknown"
	}
	reqLog = reqLog.With(
		zap.String("group_name", req.GroupName),
		zap.String("source", req.Source),
		zap.String("source_user", noteUser),
		zap.Int("validity_days", req.ValidityDays),
		zap.Float64("redeem_value", req.RedeemValue),
	)
	reqLog.Info("channel_content.generate_started")
	if req.GroupName == "" {
		reqLog.Warn("channel_content.request_validation_failed", zap.String("reason", "group_name_required"))
		response.BadRequest(c, "group_name is required")
		return
	}
	if h.groupService == nil {
		reqLog.Error("channel_content.service_unavailable", zap.String("service", "group_service"))
		response.InternalError(c, "Group service unavailable")
		return
	}
	activeGroups, err := h.groupService.ListActive(c.Request.Context())
	if err != nil {
		reqLog.Error("channel_content.load_groups_failed", zap.Error(err))
		response.InternalError(c, "Failed to load groups")
		return
	}
	reqLog.Info("channel_content.active_groups_loaded", zap.Int("active_group_count", len(activeGroups)))
	groupID, ok := findExactGroupIDByName(activeGroups, req.GroupName)
	if !ok {
		reqLog.Warn("channel_content.request_validation_failed", zap.String("reason", "invalid_group_name"))
		response.BadRequest(c, "Invalid group_name")
		return
	}
	reqLog = reqLog.With(zap.Int64("group_id", groupID))
	if h.redeemService == nil {
		reqLog.Error("channel_content.service_unavailable", zap.String("service", "redeem_service"))
		response.InternalError(c, "Redeem service unavailable")
		return
	}
	ctx := c.Request.Context()

	invitationCode, err := h.createRedeemCode(ctx, service.RedeemTypeInvitation, nil, 0, fmt.Sprintf("%s invite (%s)", req.Source, noteUser), 0)
	if err != nil {
		reqLog.Error("channel_content.generate_invitation_failed", zap.Error(err))
		response.InternalError(c, "Failed to generate invitation code")
		return
	}
	reqLog.Info("channel_content.invitation_generated", zap.String("invitation_code_masked", maskCode(invitationCode)))
	value := req.RedeemValue
	if value <= 0 {
		value = 10
	}
	validityDays := req.ValidityDays
	if validityDays <= 0 {
		validityDays = 30
	}
	redeemCode, err := h.createRedeemCode(ctx, service.RedeemTypeSubscription, &groupID, validityDays, fmt.Sprintf("%s subscription (group=%s, user=%s)", req.Source, req.GroupName, noteUser), value)
	if err != nil {
		reqLog.Error("channel_content.generate_redeem_failed", zap.Error(err), zap.Int("resolved_validity_days", validityDays), zap.Float64("resolved_redeem_value", value))
		response.InternalError(c, "Failed to generate redeem code")
		return
	}
	reqLog.Info("channel_content.redeem_generated", zap.String("redeem_code_masked", maskCode(redeemCode)), zap.Int("resolved_validity_days", validityDays), zap.Float64("resolved_redeem_value", value))

	content := h.buildReplyMessage(invitationCode, redeemCode)
	reqLog.Info("channel_content.generate_completed", zap.Int("content_length", len(content)))
	response.Success(c, GenerateChannelContentResponse{
		GroupID:        groupID,
		GroupName:      req.GroupName,
		InvitationCode: invitationCode,
		RedeemCode:     redeemCode,
		Content:        content,
	})
}

func maskCode(code string) string {
	if len(code) <= 10 {
		return code
	}
	return code[:6] + "..." + code[len(code)-4:]
}

func findExactGroupIDByName(groups []service.Group, name string) (int64, bool) {
	for _, group := range groups {
		if group.Name == name {
			return group.ID, true
		}
	}
	return 0, false
}

func (h *WeComHandler) buildReplyMessage(invitationCode, redeemCode string) string {
	var b strings.Builder
	guideURL, usageURL, notice, disclaimer := resolveContentTemplateValues(h.cfg)
	b.WriteString("教程地址为：")
	b.WriteString(guideURL)
	b.WriteString("（请复制到浏览器打开，内含详细使用方法）\n")
	b.WriteString("注册邀请码：")
	b.WriteString(invitationCode)
	b.WriteString("\n兑换码：")
	b.WriteString(redeemCode)
	b.WriteString("\n查看用量地址：")
	b.WriteString(usageURL)
	b.WriteString(" （请复制到浏览器打开，内含详细使用方法）")
	b.WriteString("\n")
	b.WriteString(notice)
	b.WriteString("\n注意： ")
	b.WriteString(disclaimer)
	return b.String()
}

func resolveContentTemplateValues(cfg *config.Config) (guideURL, usageURL, notice, disclaimer string) {
	guideURL = defaultGuideURL
	usageURL = defaultUsageURL
	notice = defaultNoticeText
	disclaimer = defaultDisclaimer
	if cfg == nil {
		return
	}
	if v := strings.TrimSpace(cfg.WeCom.Message.GuideURL); v != "" {
		guideURL = v
	}
	if v := strings.TrimSpace(cfg.WeCom.Message.UsageURL); v != "" {
		usageURL = v
	}
	if v := strings.TrimSpace(cfg.WeCom.Message.Notice); v != "" {
		notice = v
	}
	if v := strings.TrimSpace(cfg.WeCom.Message.Disclaimer); v != "" {
		disclaimer = v
	}
	return
}

func (h *WeComHandler) createRedeemCode(ctx context.Context, codeType string, groupID *int64, validityDays int, notes string, value float64) (string, error) {
	if h.redeemService == nil {
		return "", fmt.Errorf("redeem service not configured")
	}
	for i := 0; i < 5; i++ {
		code, err := service.GenerateRedeemCode()
		if err != nil {
			return "", err
		}
		redeemCode := &service.RedeemCode{
			Code:         code,
			Type:         codeType,
			Value:        value,
			Status:       service.StatusUnused,
			Notes:        notes,
			GroupID:      groupID,
			ValidityDays: validityDays,
		}
		if err := h.redeemService.CreateCode(ctx, redeemCode); err != nil {
			if ent.IsConstraintError(err) {
				continue
			}
			return "", err
		}
		return code, nil
	}
	return "", fmt.Errorf("failed to generate unique redeem code")
}
