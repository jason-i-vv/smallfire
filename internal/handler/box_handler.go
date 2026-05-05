package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// BoxHandler 箱体API处理器
type BoxHandler struct {
	boxRepo    repository.BoxRepo
	symbolRepo repository.SymbolRepo
	logger     *zap.Logger
}

// NewBoxHandler 创建箱体API处理器
func NewBoxHandler(boxRepo repository.BoxRepo, symbolRepo repository.SymbolRepo, logger *zap.Logger) *BoxHandler {
	return &BoxHandler{
		boxRepo:    boxRepo,
		symbolRepo: symbolRepo,
		logger:     logger,
	}
}

// BoxListItem 箱体列表项（包含symbol_code）
type BoxListItem struct {
	models.Box
	SymbolCode string `json:"symbol_code"`
	Trend4h    string `json:"trend_4h"`
}

// GetBoxes 获取箱体列表
func (h *BoxHandler) GetBoxes(c *gin.Context) {
	// 解析分页参数
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || size < 1 || size > 100 {
		size = 20
	}

	// 解析筛选参数
	market := c.Query("market")
	status := c.DefaultQuery("status", "active")
	boxType := c.Query("box_type")

	// 解析排序
	sortField := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	h.logger.Debug("获取箱体列表",
		zap.Int("page", page),
		zap.Int("size", size),
		zap.String("market", market),
		zap.String("status", status),
		zap.String("box_type", boxType))

	// 使用分页查询
	boxes, total, err := h.boxRepo.ListAll(page, size, status, boxType)
	if err != nil {
		h.logger.Error("获取箱体列表失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 收集所有 symbolID
	symbolIDMap := make(map[int]bool)
	for _, box := range boxes {
		symbolIDMap[box.SymbolID] = true
	}

	// 批量查询 symbol_code 和 trend_4h
	symbolCodeMap := make(map[int]string)
	symbolTrendMap := make(map[int]string)
	for symbolID := range symbolIDMap {
		if symbol, err := h.symbolRepo.GetByID(symbolID); err == nil {
			symbolCodeMap[symbolID] = symbol.SymbolCode
			symbolTrendMap[symbolID] = symbol.Trend4h
		} else {
			h.logger.Warn("查询标的失败", zap.Int("symbol_id", symbolID), zap.Error(err))
			symbolCodeMap[symbolID] = ""
		}
	}

	// 构建返回数据
	boxItems := make([]*BoxListItem, 0, len(boxes))
	for _, box := range boxes {
		boxItems = append(boxItems, &BoxListItem{
			Box:        *box,
			SymbolCode: symbolCodeMap[box.SymbolID],
			Trend4h:    symbolTrendMap[box.SymbolID],
		})
	}

	// 如果有 market 筛选，可以进一步过滤
	if market != "" {
		// 需要关联查询，暂时跳过
	}

	// 排序（在内存中排序，因为分页已经查询）
	if sortOrder == "asc" {
		switch sortField {
		case "high_price":
			for i := 0; i < len(boxItems)-1; i++ {
				for j := i + 1; j < len(boxItems); j++ {
					if boxItems[i].HighPrice > boxItems[j].HighPrice {
						boxItems[i], boxItems[j] = boxItems[j], boxItems[i]
					}
				}
			}
		case "width_percent":
			for i := 0; i < len(boxItems)-1; i++ {
				for j := i + 1; j < len(boxItems); j++ {
					if boxItems[i].WidthPercent > boxItems[j].WidthPercent {
						boxItems[i], boxItems[j] = boxItems[j], boxItems[i]
					}
				}
			}
		case "created_at":
			for i := 0; i < len(boxItems)-1; i++ {
				for j := i + 1; j < len(boxItems); j++ {
					if boxItems[i].CreatedAt.After(boxItems[j].CreatedAt) {
						boxItems[i], boxItems[j] = boxItems[j], boxItems[i]
					}
				}
			}
		}
	} else {
		switch sortField {
		case "high_price":
			for i := 0; i < len(boxItems)-1; i++ {
				for j := i + 1; j < len(boxItems); j++ {
					if boxItems[i].HighPrice < boxItems[j].HighPrice {
						boxItems[i], boxItems[j] = boxItems[j], boxItems[i]
					}
				}
			}
		case "width_percent":
			for i := 0; i < len(boxItems)-1; i++ {
				for j := i + 1; j < len(boxItems); j++ {
					if boxItems[i].WidthPercent < boxItems[j].WidthPercent {
						boxItems[i], boxItems[j] = boxItems[j], boxItems[i]
					}
				}
			}
		case "created_at":
			for i := 0; i < len(boxItems)-1; i++ {
				for j := i + 1; j < len(boxItems); j++ {
					if boxItems[i].CreatedAt.Before(boxItems[j].CreatedAt) {
						boxItems[i], boxItems[j] = boxItems[j], boxItems[i]
					}
				}
			}
		}
	}

	HandleSuccess(c, gin.H{
		"list":  boxItems,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetBoxesBySymbol 获取指定标的的箱体
func (h *BoxHandler) GetBoxesBySymbol(c *gin.Context) {
	symbolID, err := strconv.Atoi(c.Param("symbolId"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	boxes, err := h.boxRepo.GetActiveBySymbol(symbolID, "")
	if err != nil {
		h.logger.Error("获取标的箱体失败", zap.Int("symbol_id", symbolID), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, boxes)
}

// GetBox 获取箱体详情
func (h *BoxHandler) GetBox(c *gin.Context) {
	boxID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	box, err := h.boxRepo.GetByID(boxID)
	if err != nil {
		h.logger.Error("获取箱体详情失败", zap.Int("box_id", boxID), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, box)
}
