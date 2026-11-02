package v1

import (
	"github.com/chaitin/panda-wiki/domain"
	"github.com/labstack/echo/v4"
)

func learningTaskRequest(c echo.Context) (*domain.LearningTaskQuery, error) {
	var req domain.LearningTaskQuery
	if err := c.Bind(&req); err != nil {
		return nil, err
	}
	if err := c.Validate(req); err != nil {
		return nil, err
	}
	c.Response().Header().Set("Cache-Control", "private, no-store")
	return &req, nil
}

func (h *NodeHandler) LearningTaskList(c echo.Context) error {
	req, err := learningTaskRequest(c)
	if err != nil {
		return h.NewResponseWithError(c, "任务查询参数无效", err)
	}
	result, err := h.usecase.LearningTaskList(c.Request().Context(), *req)
	if err != nil {
		return h.NewResponseWithError(c, "加载后台任务失败", err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *NodeHandler) LearningTaskSummary(c echo.Context) error {
	req, err := learningTaskRequest(c)
	if err != nil {
		return h.NewResponseWithError(c, "任务查询参数无效", err)
	}
	result, err := h.usecase.LearningTaskSummary(c.Request().Context(), req.KBID)
	if err != nil {
		return h.NewResponseWithError(c, "加载后台任务统计失败", err)
	}
	return h.NewResponseWithData(c, result)
}

func (h *NodeHandler) LearningTaskTargets(c echo.Context) error {
	req, err := learningTaskRequest(c)
	if err != nil {
		return h.NewResponseWithError(c, "任务查询参数无效", err)
	}
	ids, err := h.usecase.LearningTaskTargets(c.Request().Context(), *req)
	if err != nil {
		return h.NewResponseWithError(c, "获取筛选文档失败", err)
	}
	return h.NewResponseWithData(c, map[string]any{"ids": ids})
}

func (h *NodeHandler) LearningTaskDetail(c echo.Context) error {
	req, err := learningTaskRequest(c)
	if err != nil {
		return h.NewResponseWithError(c, "任务查询参数无效", err)
	}
	id := c.QueryParam("id")
	if id == "" {
		return h.NewResponseWithError(c, "缺少文档 ID", nil)
	}
	result, err := h.usecase.LearningTaskDetail(c.Request().Context(), req.KBID, id)
	if err != nil {
		return h.NewResponseWithError(c, "无法读取该文档的任务进度", err)
	}
	return h.NewResponseWithData(c, result)
}
