package permission

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Sakurame1/frp-manager/common"
	"github.com/Sakurame1/frp-manager/defs"
	"github.com/Sakurame1/frp-manager/models"
	"github.com/Sakurame1/frp-manager/services/app"
	"github.com/Sakurame1/frp-manager/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type batchUsersRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Count    int    `json:"count"`
}

var numberedName = regexp.MustCompile(`^(.*?)([0-9]+)$`)

func batchIdentities(req batchUsersRequest) ([]models.UserEntity, error) {
	if req.Count < 1 || req.Count > 100 {
		return nil, fmt.Errorf("每批数量必须为 1–100")
	}
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(req.Email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("请输入有效的起始邮箱")
	}
	nameMatch, mailMatch := numberedName.FindStringSubmatch(username), numberedName.FindStringSubmatch(parts[0])
	if nameMatch == nil || mailMatch == nil {
		return nil, fmt.Errorf("起始用户名和邮箱前缀必须以数字结尾，例如 ig01")
	}
	start, err := strconv.Atoi(nameMatch[2])
	if err != nil || start > 999999999-req.Count {
		return nil, fmt.Errorf("用户名编号过大")
	}
	mailStart, err := strconv.Atoi(mailMatch[2])
	if err != nil || mailStart > 999999999-req.Count {
		return nil, fmt.Errorf("邮箱编号过大")
	}
	rows := make([]models.UserEntity, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		name := fmt.Sprintf("%s%0*d", nameMatch[1], len(nameMatch[2]), start+i)
		address := fmt.Sprintf("%s%0*d@%s", mailMatch[1], len(mailMatch[2]), mailStart+i, parts[1])
		if err = utils.ValidateUserName(name); err != nil {
			return nil, err
		}
		if err = utils.ValidateEmail(address); err != nil {
			return nil, err
		}
		if len(address) > 72 {
			return nil, fmt.Errorf("邮箱过长，不能作为初始密码（最多 72 字节）")
		}
		rows = append(rows, models.UserEntity{UserName: name, Email: address, MustChangePassword: true, Status: models.STATUS_NORMAL, Role: defs.UserRole_Normal})
	}
	return rows, nil
}

func createBatchUsers(db *gorm.DB, tenantID int, req batchUsersRequest) ([]models.UserEntity, error) {
	rows, err := batchIdentities(req)
	if err != nil {
		return nil, err
	}
	names, emails := make([]string, 0, len(rows)), make([]string, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.UserName)
		emails = append(emails, row.Email)
	}
	// Hash outside the write transaction, so a large batch does not lock the
	// users table while calculating password hashes.
	var conflict int64
	if err := db.Unscoped().Model(&models.User{}).Where("user_name IN ? OR email IN ?", names, emails).Count(&conflict).Error; err != nil {
		return nil, err
	}
	if conflict > 0 {
		return nil, fmt.Errorf("用户名或邮箱已存在，整批未创建，请调整起始编号")
	}
	for i := range rows {
		password, err := utils.HashPassword(rows[i].Email)
		if err != nil {
			return nil, err
		}
		rows[i].Password = password
		rows[i].TenantID = tenantID
		rows[i].Token = uuid.NewString()
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		var existing models.User
		if err := tx.Unscoped().Where("user_name IN ? OR email IN ?", names, emails).Limit(1).Find(&existing).Error; err != nil {
			return err
		}
		if existing.UserEntity != nil {
			return fmt.Errorf("用户名或邮箱已存在，整批未创建，请调整起始编号")
		}
		for i := range rows {
			if err = tx.Create(&models.User{UserEntity: &rows[i]}).Error; err != nil {
				return fmt.Errorf("批量创建失败，整批已回滚")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i] = rows[i].GetSafeUserInfo()
	}
	return rows, nil
}

func BatchCreateUsers(instance app.Application) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := common.GetUserInfo(c)
		if user == nil || !user.IsAdmin() {
			errJSON(c, http.StatusForbidden, fmt.Errorf("仅管理员可以批量创建用户"))
			return
		}
		var req batchUsersRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			errJSON(c, http.StatusBadRequest, fmt.Errorf("invalid request"))
			return
		}
		rows, err := createBatchUsers(instance.GetDBManager().GetDefaultDB(), user.GetTenantID(), req)
		if err != nil {
			errJSON(c, http.StatusBadRequest, err)
			return
		}
		okJSON(c, rows)
	}
}
