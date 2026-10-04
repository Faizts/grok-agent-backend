package logger

import "go.uber.org/zap"

var L *zap.Logger

func Init(dev bool) {
	var err error
	if dev {
		L, err = zap.NewDevelopment()
	} else {
		L, err = zap.NewProduction()
	}
	if err != nil {
		panic(err)
	}
}

func Sugar() *zap.SugaredLogger {
	return L.Sugar()
}
