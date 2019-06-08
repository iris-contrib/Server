package services

import (
	"github.com/TimeForCoin/Server/app/libs"
	"github.com/TimeForCoin/Server/app/models"
	"github.com/kataras/iris"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sort"
	"strings"
)

// TaskService 用户逻辑
type TaskService interface {
	AddTask(userID primitive.ObjectID, info models.TaskSchema, publish bool)
	SetTaskInfo(userID, taskID primitive.ObjectID, info models.TaskSchema)
	// SetTaskFile(taskID primitive.ObjectID, files []primitive.ObjectID)
	GetTaskByID(taskID primitive.ObjectID, userID string) (task TaskDetail)
	GetTasks(page, size int64, sortRule, taskType,
		status , reward , keyword, user, userID string) (taskCount int64, tasks []TaskDetail)
}

// NewUserService 初始化
func newTaskService() TaskService {
	return &taskService{
		model:     models.GetModel().Task,
		userModel: models.GetModel().User,
		fileModel: models.GetModel().File,
		cache:     models.GetRedis().Cache,
	}
}

type ImagesData struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type omit *struct{}
type TaskDetail struct {
	*models.TaskSchema
	// 额外项
	Publisher  models.UserBaseInfo
	Attachment []models.FileSchema
	Images     []ImagesData
	Liked      bool
	Collected  bool
	// 排除项
	LikeID omit `json:"like_id,omitempty"` // 点赞用户ID
}

type taskService struct {
	model     *models.TaskModel
	userModel *models.UserModel
	fileModel *models.FileModel
	cache     *models.CacheModel
}

func (s *taskService) AddTask(userID primitive.ObjectID, info models.TaskSchema, publish bool) {
	status := models.TaskStatusDraft
	if publish {
		status = models.TaskStatusWait
	}
	id, err := s.model.AddTask(userID, status)
	libs.AssertErr(err, "", iris.StatusInternalServerError)
	err = s.model.SetTaskInfoByID(id, info)
	libs.AssertErr(err, "", iris.StatusInternalServerError)
}

func (s *taskService) SetTaskInfo(userID, taskID primitive.ObjectID, info models.TaskSchema) {
	task, err := s.model.GetTaskByID(taskID)
	libs.AssertErr(err, "faked_task", 403)
	libs.Assert(task.Publisher == userID, "permission_deny", 403)

	libs.Assert(task.Status == models.TaskStatusDraft ||
		task.Status == models.TaskStatusWait, "not_allow_edit", 403)

	libs.Assert(string(info.Status) == "" ||
		info.Status == models.TaskStatusWait  ||
		info.Status == models.TaskStatusClose ||
		info.Status == models.TaskStatusFinish, "not_allow_status", 403)
	if info.Status == models.TaskStatusWait {
		libs.Assert(task.Status == models.TaskStatusDraft, "not_allow_status", 403)
	} else if info.Status == models.TaskStatusClose || info.Status == models.TaskStatusFinish {
		libs.Assert(task.Status == models.TaskStatusWait, "not_allow_status", 403)
	}

	libs.Assert(info.MaxPlayer == 0 || info.MaxPlayer > task.PlayerCount, "not_allow_max_player", 403)

	if task.Status != models.TaskStatusDraft && task.Reward != models.RewardObject {
		libs.Assert(info.RewardValue > task.RewardValue, "not_allow_reward_value", 403)
	}

	//libs.Assert(task.Status == models.TaskStatusDraft ||
	//	task.Status == models.TaskStatusWait ||
	//	task.Status == models.TaskStatusRun, "not_allow_edit", 403)


	err = s.model.SetTaskInfoByID(taskID, info)
	libs.AssertErr(err, "", iris.StatusInternalServerError)
}

func (s *taskService) GetTaskByID(taskID primitive.ObjectID, userID string) (task TaskDetail) {
	var err error
	taskItem, err := s.model.GetTaskByID(taskID)
	libs.AssertErr(err, "faked_task", 403)
	task.TaskSchema = &taskItem

	user, err := s.cache.GetUserBaseInfo(taskItem.Publisher)
	libs.AssertErr(err, "", iris.StatusInternalServerError)
	task.Publisher = user

	images, err := s.fileModel.GetFileByContent(taskID, models.FileImage)
	libs.AssertErr(err, "", iris.StatusInternalServerError)

	task.Images = []ImagesData{}
	for _, i := range images {
		task.Images = append(task.Images, ImagesData{
			ID:  i.ID.Hex(),
			URL: i.URL,
		})
	}

	attachment, err := s.fileModel.GetFileByContent(taskID, models.FileFile)
	libs.AssertErr(err, "", iris.StatusInternalServerError)
	if attachment == nil {
		attachment = []models.FileSchema{}
	}
	task.Attachment = attachment

	if userID != "" {
		id, err := primitive.ObjectIDFromHex(userID)
		if err != nil {
			task.Liked = s.cache.IsLikeTask(id, task.ID)
			task.Collected = s.cache.IsCollectTask(id, task.ID)
		}
	}

	return
}

// 分页获取任务列表，需要按类型/状态/酬劳类型/用户类型筛选，按关键词搜索，按不同规则排序
func (s *taskService) GetTasks(page, size int64, sortRule, taskType,
	status , reward , keyword, user, userID string) (taskCount int64, taskCards []TaskDetail) {

	var taskTypes []models.TaskType
	split := strings.Split(taskType, ",")
	sort.Strings(split)
	if sort.SearchStrings(split, "all") != -1 || sortRule == "user" {
		taskTypes = []models.TaskType{models.TaskTypeRunning, models.TaskTypeQuestionnaire, models.TaskTypeInfo}
	} else {
		for _, str := range split {
			taskTypes = append(taskTypes, models.TaskType(str))
		}
	}

	var statuses []models.TaskStatus
	split = strings.Split(status, ",")
	sort.Strings(split)
	if sort.SearchStrings(split, "all") != -1 || sortRule == "user" {
		statuses = []models.TaskStatus{models.TaskStatusClose, models.TaskStatusFinish,
			models.TaskStatusWait}
	} else {
		for _, str := range split {
			statuses = append(statuses, models.TaskStatus(str))
		}
	}

	var rewards []models.RewardType
	split = strings.Split(reward, ",")
	sort.Strings(split)
	if sort.SearchStrings(split, "all") != -1 || sortRule == "user" {
		rewards = []models.RewardType{models.RewardMoney, models.RewardObject, models.RewardRMB}
	} else {
		for _, str := range split {
			rewards = append(rewards, models.RewardType(str))
		}
	}

	keywords := strings.Split(keyword, " ")

	if sortRule == "new" {
		sortRule = "publish_date"
	}

	tasks, taskCount, err := s.model.GetTasks(sortRule, taskTypes, statuses, rewards, keywords, user, (page - 1) * size, size)
	libs.AssertErr(err, "", iris.StatusInternalServerError)

	for i, t := range tasks {
		var task TaskDetail
		task.TaskSchema = &tasks[i]

		user, err := s.cache.GetUserBaseInfo(t.Publisher)
		libs.AssertErr(err, "", iris.StatusInternalServerError)
		task.Publisher = user

		images, err := s.fileModel.GetFileByContent(t.ID, models.FileImage)
		libs.AssertErr(err, "", iris.StatusInternalServerError)
		task.Images = []ImagesData{}
		task.Attachment = []models.FileSchema{}
		for _, i := range images {
			task.Images = append(task.Images, ImagesData{
				ID:  i.ID.Hex(),
				URL: i.URL,
			})
		}
		if userID != "" {
			id, err := primitive.ObjectIDFromHex(userID)
			if err != nil {
				task.Liked = s.cache.IsLikeTask(id, t.ID)
				task.Collected = s.cache.IsCollectTask(id, t.ID)
			}
		}

		taskCards = append(taskCards, task)
	}

	return
}
