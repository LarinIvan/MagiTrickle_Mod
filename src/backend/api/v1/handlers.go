package v1

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	// Added by user instruction
	"magitrickle/api/utils"
	"magitrickle/api/v1/types"
	"magitrickle/app"
	"magitrickle/diagnostics"
	"magitrickle/models"
	"magitrickle/utils/intID"
	"magitrickle/utils/network"
	"magitrickle/utils/updater"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

// Handler предоставляет набор методов для обработки API запросов.
type Handler struct {
	app app.Main
}

// NewHandler создаёт новый обработчик для API v1.
func NewHandler(a app.Main) *Handler {
	return &Handler{app: a}
}

// NetfilterDHook
//
//	@Summary		Хук эвента netfilter.d
//	@Description	Эмитирует хук эвента netfilter.d
//	@Tags			hooks
//	@Accept			json
//	@Produce		json
//	@Param			json	body		types.NetfilterDHookReq	true	"Тело запроса"
//	@Success		200
//	@Failure		400		{object}	types.ErrorRes
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/hooks/netfilterd [post]
func (h *Handler) NetfilterDHook(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.NetfilterDHookReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	log.Debug().
		Str("type", req.Type).
		Str("table", req.Table).
		Msg("received netfilter.d event")
	if h.app.DnsOverrider() != nil {
		if err := h.app.DnsOverrider().NetfilterDHook(req.Type, req.Table); err != nil {
			log.Error().Err(err).Msg("error fixing iptables after netfilter.d")
		}
	}
	if err := h.app.NetfilterDHook(req.Type, req.Table); err != nil {
		log.Error().Err(err).Msg("error fixing traffic manager rules after netfilter.d")
	}
}

// ListInterfaces
//
//	@Summary		Получить список интерфейсов
//	@Description	Возвращает список интерфейсов
//	@Tags			config
//	@Produce		json
//	@Success		200		{object}	types.InterfacesRes
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/interfaces [get]
func (h *Handler) ListInterfaces(w http.ResponseWriter, r *http.Request) {
	interfaces, err := h.app.ListInterfaces()
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Errorf("failed to get interfaces: %w", err).Error())
		return
	}
	res := make([]types.InterfaceRes, len(interfaces)+1)
	res[0] = types.InterfaceRes{ID: "blackhole", Active: true}
	for i, iface := range interfaces {
		active := iface.Flags&net.FlagUp != 0
		ip := ""
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ip = ipnet.IP.String()
					break
				}
			}
		}
		res[i+1] = types.InterfaceRes{ID: iface.Name, Active: active, IP: ip}
	}
	utils.WriteJson(w, http.StatusOK, types.InterfacesRes{Interfaces: res})
}

// SaveConfig
//
//	@Summary		Сохранить конфигурацию
//	@Description	Сохраняет текущую конфигурацию в постоянную память
//	@Tags			config
//	@Produce		json
//	@Success		200
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/config/save [post]
func (h *Handler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if err := h.app.SaveConfig(); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
	}
}

// GetGroups
//
//	@Summary		Получить список групп
//	@Description	Возвращает список групп
//	@Tags			groups
//	@Produce		json
//	@Param			with_rules	query		bool	false	"Возвращать группы с их правилами"
//	@Success		200			{object}	types.GroupsRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups [get]
func (h *Handler) GetGroups(w http.ResponseWriter, r *http.Request) {
	withRules := r.URL.Query().Get("with_rules") == "true"
	appGroups := h.app.Groups()
	modelGroups := make([]*models.Group, len(appGroups))
	for i, g := range appGroups {
		modelGroups[i] = g.Model()
	}
	utils.WriteJson(w, http.StatusOK, RespFromGroups(modelGroups, withRules))
}

// PutGroups
//
//	@Summary		Обновить список групп
//	@Description	Обновляет список групп
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			save	query		bool			false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		types.GroupsReq	true	"Тело запроса"
//	@Success		200			{object}	types.GroupsRes
//	@Failure		400			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups [put]
func (h *Handler) PutGroups(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.GroupsReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Groups == nil {
		utils.WriteError(w, http.StatusBadRequest, "no groups in request")
		return
	}
	for _, g := range h.app.Groups() {
		_ = g.Disable()
	}
	newGroups := make([]*models.Group, len(*req.Groups))
	for i, gReq := range *req.Groups {
		var existing *models.Group
		for _, g := range h.app.Groups() {
			if gReq.ID != nil && g.Model().ID == *gReq.ID {
				existing = g.Model()
				break
			}
		}
		newGroups[i], err = GroupFromReq(gReq, existing)
		if err != nil {
			utils.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	h.app.ClearGroups()
	for _, grp := range newGroups {
		if err := h.app.AddGroup(grp); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, RespFromGroups(newGroups, true))
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// CreateGroup
//
//	@Summary		Создать группу
//	@Description	Создает группу
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			save	query		bool			false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		types.GroupReq	true	"Тело запроса"
//	@Success		200			{object}	types.GroupRes
//	@Failure		400			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups [post]
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.GroupReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	group, err := GroupFromReq(req, nil)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.app.AddGroup(group); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.WriteJson(w, http.StatusOK, RespFromGroup(group, true))
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// GetGroup
//
//	@Summary		Получить группу
//	@Description	Возвращает запрошенную группу
//	@Tags			groups
//	@Produce		json
//	@Param			groupID		path		string	true	"ID группы"
//	@Param			with_rules	query		bool	false	"Возвращать группу с её правилами"
//	@Success		200			{object}	types.GroupRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID} [get]
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	withRules := r.URL.Query().Get("with_rules") == "true"
	group := h.app.Groups()[groupIdx].Model()
	utils.WriteJson(w, http.StatusOK, RespFromGroup(group, withRules))
}

// PutGroup
//
//	@Summary		Обновить группу
//	@Description	Обновляет запрошенную группу
//	@Tags			groups
//	@Accept			json
//	@Produce		json
//	@Param			groupID	path		string			true	"ID группы"
//	@Param			save	query		bool			false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		types.GroupReq	true	"Тело запроса"
//	@Success		200			{object}	types.GroupRes
//	@Failure		400			{object}	types.ErrorRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID} [put]
func (h *Handler) PutGroup(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.GroupReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	groupWrapper := h.app.Groups()[groupIdx]

	enabled := groupWrapper.Enabled()
	if enabled {
		if err := groupWrapper.Disable(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to disable group: %v", err))
			return
		}
	}

	updatedGroup, err := GroupFromReq(req, groupWrapper.Model())
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if enabled {
		if err := groupWrapper.Enable(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to enable group: %v", err))
			return
		}
		if err := groupWrapper.Sync(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to sync group: %v", err))
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, RespFromGroup(updatedGroup, true))
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// DeleteGroup
//
//	@Summary		Удалить группу
//	@Description	Удаляет запрошенную группу
//	@Tags			groups
//	@Produce		json
//	@Param			groupID	path		string	true	"ID группы"
//	@Param			save	query		bool	false	"Сохранить изменения в конфигурационный файл"
//	@Success		200
//	@Failure		404		{object}	types.ErrorRes
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID} [delete]
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	groupWrapper := h.app.Groups()[groupIdx]
	if groupWrapper.Enabled() {
		if err := groupWrapper.Disable(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to disable group: %v", err))
			return
		}
	}
	h.app.RemoveGroupByIndex(groupIdx)
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// GetRules
//
//	@Summary		Получить список правил
//	@Description	Возвращает список правил
//	@Tags			rules
//	@Produce		json
//	@Param			groupID	path		string	true	"ID группы"
//	@Success		200			{object}	types.RulesRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID}/rules [get]
func (h *Handler) GetRules(w http.ResponseWriter, r *http.Request) {
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	rules := h.app.Groups()[groupIdx].Model().Rules
	utils.WriteJson(w, http.StatusOK, RespFromRules(rules))
}

// PutRules
//
//	@Summary		Обновить список правил
//	@Description	Обновляет список правил
//	@Tags			rules
//	@Accept			json
//	@Produce		json
//	@Param			groupID	path		string			true	"ID группы"
//	@Param			save	query		bool			false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		types.RulesReq	true	"Тело запроса"
//	@Success		200			{object}	types.RulesRes
//	@Failure		400			{object}	types.ErrorRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID}/rules [put]
func (h *Handler) PutRules(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.RulesReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Rules == nil {
		utils.WriteError(w, http.StatusBadRequest, "no rules in request")
		return
	}
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	groupWrapper := h.app.Groups()[groupIdx]
	enabled := groupWrapper.Enabled()

	newRules := make([]*models.Rule, len(*req.Rules))
	for i, rr := range *req.Rules {
		id := intID.RandomID()
		if rr.ID != nil {
			found := false
			for _, oldRule := range groupWrapper.Model().Rules {
				if oldRule.ID == *rr.ID {
					id = *rr.ID
					found = true
					break
				}
			}
			if !found {
				utils.WriteError(w, http.StatusNotFound, "rule not found")
				return
			}
		}
		newRules[i] = &models.Rule{
			ID:     id,
			Name:   rr.Name,
			Type:   rr.Type,
			Rule:   rr.Rule,
			Enable: rr.Enable,
		}
	}
	groupWrapper.Model().Rules = newRules
	if enabled {
		if err := groupWrapper.Sync(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to sync group: %v", err))
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, RespFromRules(newRules))
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// CreateRule
//
//	@Summary		Создать правило
//	@Description	Создает правило
//	@Tags			rules
//	@Accept			json
//	@Produce		json
//	@Param			groupID	path		string			true	"ID группы"
//	@Param			save	query		bool			false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		types.RuleReq	true	"Тело запроса"
//	@Success		200			{object}	types.RuleRes
//	@Failure		400			{object}	types.ErrorRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID}/rules [post]
func (h *Handler) CreateRule(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.RuleReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	groupWrapper := h.app.Groups()[groupIdx]
	enabled := groupWrapper.Enabled()

	rule, err := RuleFromReq(req, groupWrapper.Model().Rules)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	groupWrapper.Model().Rules = append(groupWrapper.Model().Rules, rule)
	if enabled {
		if err := groupWrapper.Sync(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to sync group: %v", err))
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, RespFromRule(rule))
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// GetRule
//
//	@Summary		Получить правило
//	@Description	Возвращает запрошенное правило
//	@Tags			rules
//	@Produce		json
//	@Param			groupID	path		string	true	"ID группы"
//	@Param			ruleID	path		string	true	"ID правила"
//	@Success		200			{object}	types.RuleRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID}/rules/{ruleID} [get]
func (h *Handler) GetRule(w http.ResponseWriter, r *http.Request) {
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	ruleIdx, _ := strconv.Atoi(r.Header.Get("ruleIdx"))
	rule := h.app.Groups()[groupIdx].Model().Rules[ruleIdx]
	utils.WriteJson(w, http.StatusOK, RespFromRule(rule))
}

// PutRule
//
//	@Summary		Обновить правило
//	@Description	Обновляет запрошенное правило
//	@Tags			rules
//	@Accept			json
//	@Produce		json
//	@Param			groupID	path		string			true	"ID группы"
//	@Param			ruleID	path		string			true	"ID правила"
//	@Param			save	query		bool			false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		types.RuleReq	true	"Тело запроса"
//	@Success		200			{object}	types.RuleRes
//	@Failure		400			{object}	types.ErrorRes
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID}/rules/{ruleID} [put]
func (h *Handler) PutRule(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[types.RuleReq](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	groupWrapper := h.app.Groups()[groupIdx]
	enabled := groupWrapper.Enabled()

	ruleIdx, _ := strconv.Atoi(r.Header.Get("ruleIdx"))
	rule := groupWrapper.Model().Rules[ruleIdx]
	rule.Name = req.Name
	rule.Type = req.Type
	rule.Rule = req.Rule
	rule.Enable = req.Enable

	if enabled {
		if err := groupWrapper.Sync(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to sync group: %v", err))
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, RespFromRule(rule))
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// DeleteRule
//
//	@Summary		Удалить правило
//	@Description	Удаляет запрошенное правило
//	@Tags			rules
//	@Produce		json
//	@Param			groupID	path		string	true	"ID группы"
//	@Param			ruleID	path		string	true	"ID правила"
//	@Param			save	query		bool	false	"Сохранить изменения в конфигурационный файл"
//	@Success		200
//	@Failure		404			{object}	types.ErrorRes
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/groups/{groupID}/rules/{ruleID} [delete]
func (h *Handler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	groupIdx, _ := strconv.Atoi(r.Header.Get("groupIdx"))
	groupWrapper := h.app.Groups()[groupIdx]
	enabled := groupWrapper.Enabled()

	ruleIdx, _ := strconv.Atoi(r.Header.Get("ruleIdx"))
	groupWrapper.Model().Rules = append(groupWrapper.Model().Rules[:ruleIdx], groupWrapper.Model().Rules[ruleIdx+1:]...)
	if enabled {
		if err := groupWrapper.Sync(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to sync group: %v", err))
			return
		}
	}
	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config file: %v", err))
			return
		}
	}
}

// ListInterfaceAliases
//
//	@Summary		Получить список алиасов интерфейсов
//	@Description	Возвращает список алиасов интерфейсов
//	@Tags			config
//	@Produce		json
//	@Success		200		{object}	map[string]string
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/interfaces/aliases [get]
func (h *Handler) ListInterfaceAliases(w http.ResponseWriter, r *http.Request) {
	utils.WriteJson(w, http.StatusOK, h.app.InterfaceAliases())
}

// SaveInterfaceAliases
//
//	@Summary		Сохранить алиасы интерфейсов
//	@Description	Сохраняет алиасы интерфейсов
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Param			save	query		bool				false	"Сохранить изменения в конфигурационный файл"
//	@Param			json	body		map[string]string	true	"Тело запроса"
//	@Success		200
//	@Failure		400		{object}	types.ErrorRes
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/interfaces/aliases [post]
func (h *Handler) SaveInterfaceAliases(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[map[string]string](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.app.SetInterfaceAliases(req)

	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveInterfaceConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save config: %v", err))
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, nil)
}

// ListSettings
//
//	@Summary		Получить системные настройки
//	@Description	Возвращает системные настройки
//	@Tags			config
//	@Produce		json
//	@Success		200		{object}	models.SettingsConfig
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/settings [get]
//
//	func (h *Handler) ListSettings(w http.ResponseWriter, r *http.Request) {
//		utils.WriteJson(w, http.StatusOK, h.app.Settings())
//	}
func (h *Handler) ListSettings(w http.ResponseWriter, r *http.Request) {
	utils.WriteJson(w, http.StatusOK, h.app.Settings())
}

// SaveSettings
//
//	@Summary		Сохранить системные настройки
//	@Description	Сохраняет системные настройки
//	@Tags			config
//	@Accept			json
//	@Produce		json
//	@Param			save	query		bool					false	"Сохранить изменения в файл"
//	@Param			json	body		models.SettingsConfig	true	"Тело запроса"
//	@Success		200
//	@Failure		400		{object}	types.ErrorRes
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/settings [post]
//
//	func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
//		req, err := utils.ReadJson[models.SettingsConfig](r)
//		if err != nil {
//			utils.WriteError(w, http.StatusBadRequest, err.Error())
//			return
//		}
//		h.app.SetSettings(req)
//
//		if r.URL.Query().Get("save") == "true" {
//			if err := h.app.SaveSettingsConfig(); err != nil {
//				utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save settings: %v", err))
//				return
//			}
//		}
//		utils.WriteJson(w, http.StatusOK, nil)
//	}
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	req, err := utils.ReadJson[models.SettingsConfig](r)
	if err != nil {
		utils.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.app.SetSettings(req)

	if r.URL.Query().Get("save") == "true" {
		if err := h.app.SaveSettingsConfig(); err != nil {
			utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save settings: %v", err))
			return
		}
	}
	utils.WriteJson(w, http.StatusOK, nil)
}

// RestartService
//
//	@Summary		Перезагрузить сервис
//	@Description	Перезагружает сервис
//	@Tags			system
//	@Produce		json
//	@Success		200
//	@Router			/api/v1/system/restart [post]
func (h *Handler) RestartService(w http.ResponseWriter, r *http.Request) {
	h.app.Restart()
	utils.WriteJson(w, http.StatusOK, map[string]string{"status": "restarting"})
}

// StreamLogs
//
//	@Summary		Стрим логов (SSE)
//	@Description	Стримит логи приложения в реальном времени
//	@Tags			system
//	@Produce		text/event-stream
//	@Success		200
//	@Router			/api/v1/system/logs/stream [get]
func (h *Handler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	broadcaster := h.app.LogBroadcaster()
	if broadcaster == nil {
		http.Error(w, "Logs broadcaster not available", http.StatusInternalServerError)
		return
	}

	// msgChan := make(chan []byte, 100)
	msgChan := broadcaster.Subscribe()
	defer broadcaster.Unsubscribe(msgChan)

	clientDisconnected := r.Context().Done()

	for {
		select {
		case msg := <-msgChan:
			// SSE format: "data: <payload>\n\n"
			// We assume msg is a JSON line from zerolog
			fmt.Fprintf(w, "data: %s\n\n", msg)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-clientDisconnected:
			return
		}
	}
}

// GetExternalIP
//
//	@Summary		Получить внешний IP для интерфейса
//	@Description	Проверяет внешний IP для конкретного интерфейса
//	@Tags			config
//	@Produce		json
//	@Param			iface	path		string	true	"Имя интерфейса"
//	@Success		200		{object}	map[string]string
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/interfaces/{iface}/external-ip [get]
func (h *Handler) GetExternalIP(w http.ResponseWriter, r *http.Request) {
	ifaceName := chi.URLParam(r, "iface")
	if ifaceName == "" {
		utils.WriteError(w, http.StatusBadRequest, "interface name is required")
		return
	}

	ip, err := network.CheckExternalIP(ifaceName)
	if err != nil {
		// Return 200 OK with empty IP to avoid browser console errors (500)
		// The frontend simply hides the IP if it's empty.
		utils.WriteJson(w, http.StatusOK, map[string]string{
			"ip":    "",
			"error": err.Error(),
		})
		return
	}

	utils.WriteJson(w, http.StatusOK, map[string]string{"ip": ip})
}

// RunSpeedtest
//
//	@Summary		Запустить тест скорости (Stream)
//	@Description	Запускает Speedtest на указанном интерфейсе и стримит результаты (SSE)
//	@Tags			diagnostics
//	@Produce		text/event-stream
//	@Param			interface	query		string	false	"Имя интерфейса"
//	@Success		200
//	@Success		200
//	@Router			/api/v1/diagnostics/speedtest [get]
func (h *Handler) RunSpeedtest(w http.ResponseWriter, r *http.Request) {
	ifaceName := r.URL.Query().Get("interface")
	serverID, _ := strconv.Atoi(r.URL.Query().Get("server_id"))
	parallelLoss := r.URL.Query().Get("parallel_loss") == "true"
	diagnostics.RunSpeedtestStream(r.Context(), w, ifaceName, serverID, parallelLoss)
}

// GetSpeedtestServers
//
//	@Summary		Получить список серверов Speedtest
//	@Description	Возвращает список доступных серверов Speedtest
//	@Tags			diagnostics
//	@Produce		json
//	@Param			interface	query		string	false	"Имя интерфейса"
//	@Success		200			{object}	[]types.SpeedtestServer
//	@Failure		500			{object}	types.ErrorRes
//	@Router			/api/v1/diagnostics/speedtest/servers [get]
func (h *Handler) GetSpeedtestServers(w http.ResponseWriter, r *http.Request) {
	ifaceName := r.URL.Query().Get("interface")
	search := r.URL.Query().Get("search")
	servers, err := diagnostics.GetServers(ifaceName, search)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get servers: %v", err))
		return
	}

	// Convert to DTO if needed using simple anonymous struct or define type
	// Since we don't have a specific DTO type readily available in the import list for handlers,
	// let's create a simplified response or map direct fields.
	// Ideally we should use a type from api/v1/types.
	// Let's assume we return simplified struct list.

	res := make([]map[string]interface{}, len(servers))
	for i, s := range servers {
		res[i] = map[string]interface{}{
			"id":       s.ID,
			"name":     s.Name,
			"country":  s.Country,
			"sponsor":  s.Sponsor,
			"distance": s.Distance,
			"latency":  s.Latency.Milliseconds(),
		}
	}
	utils.WriteJson(w, http.StatusOK, res)
}

// CheckUpdate
//
//	@Summary		Проверить наличие обновлений
//	@Description	Запускает проверку обновлений через opkg
//	@Tags			system
//	@Produce		json
//	@Success		200		{object}	map[string]string
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/update/check [get]
func (h *Handler) CheckUpdate(w http.ResponseWriter, r *http.Request) {
	newVer, err := updater.CheckForUpdates()
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to check for updates: %v", err))
		return
	}
	utils.WriteJson(w, http.StatusOK, map[string]string{"available_version": newVer})
}

// RunUpdate
//
//	@Summary		Запустить обновление
//	@Description	Запускает процесс обновления (сервис будет перезагружен)
//	@Tags			system
//	@Produce		json
//	@Success		200
//	@Failure		500		{object}	types.ErrorRes
//	@Router			/api/v1/system/update/run [post]
func (h *Handler) RunUpdate(w http.ResponseWriter, r *http.Request) {
	if err := updater.RunUpdate(); err != nil {
		utils.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("failed to start update: %v", err))
		return
	}
	utils.WriteJson(w, http.StatusOK, nil)
}
