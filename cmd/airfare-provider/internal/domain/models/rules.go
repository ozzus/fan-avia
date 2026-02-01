package models

import "time"

type Direction uint8

const (
	DirectionUnspecified Direction = iota
	DirectionOutbound
	DirectionReturn
)

type RuleType uint8

const (
	RuleTypeUnspecified RuleType = iota
	RuleOutDMinus2
	RuleOutDMinus1
	RuleOutArriveBy
	RuleRetDepartAfter
	RuleRetDPlus1
	RuleRetDPlus2
)

type TimeConstraint struct {
	From time.Time
	To   time.Time
}

type Rule struct {
	Type           RuleType
	Direction      Direction
	DayUTC         time.Time
	TimeConstraint TimeConstraint
}
