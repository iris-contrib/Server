package models

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFileModel(t *testing.T) {
	t.Run("InitDB", testInitDB)
	t.Run("testAddFile", testAddFile)

	ctx, finish := GetCtx()
	defer finish()
	err := model.File.Collection.Drop(ctx)
	if err != nil {
		t.Error(err)
	}

	t.Run("DisconnectDB", testDisconnectDB)
}

func testAddFile(t *testing.T) {
	newID := primitive.NewObjectID()
	res, err := GetModel().File.GetFile(newID)
	if err == nil {
		t.Error(res)
	}
	err = GetModel().File.AddFile(newID, newID, FileForTask, FileImage,
		"文件", "附件", "https://xx.com/a.zip", 10085, true, true)
	if err != nil {
		t.Error(err)
	}

	res, err = GetModel().File.GetFile(newID)
	if err != nil {
		t.Error(err)
	}
	t.Log(res)
}
