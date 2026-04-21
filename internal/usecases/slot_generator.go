package usecases

import (
	"fmt"
	"time"

	"room-booking/internal/models"
)

const SlotDuration = 30 * time.Minute

// GenerateSlots нарезает время на куски по 30 минут для конкретного дня недели
// на ближайшие windowDaysб, например 30 дней
func GenerateSlots(roomID string, dayOfWeek int, startTimeStr, endTimeStr string, windowDays int) ([]models.Slot, error) {
	var slots []models.Slot

	// Парсим время начала и конца (ожидаем формат "HH:MM")
	startTime, err := time.Parse("15:04", startTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start time format")
	}
	endTime, err := time.Parse("15:04", endTimeStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end time format")
	}

	if !endTime.After(startTime) {
		return nil, fmt.Errorf("end time must be after start time")
	}

	// Конвертируем день недели из API (1=Пн, 7=Вс) в формат Go (0=Вс, 1=Пн)
	targetWeekday := time.Weekday(dayOfWeek % 7)

	// Берем начало сегодняшнего дня в UTC
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	// Проходимся по ближайшим N дням
	for i := 0; i < windowDays; i++ {
		currentDate := today.AddDate(0, 0, i)

		// Если текущий день совпадает с днем расписания
		if currentDate.Weekday() == targetWeekday {
			// Устанавливаем конкретное время начала и конца для этого дня
			startOfSlot := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
			endOfSchedule := time.Date(currentDate.Year(), currentDate.Month(), currentDate.Day(), endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

			// Нарезаем слоты по 30 минут, пока не упремся в endOfSchedule
			for {
				endOfSlot := startOfSlot.Add(SlotDuration)
				if endOfSlot.After(endOfSchedule) {
					break // Слот не влезает в расписание, останавливаемся
				}

				slots = append(slots, models.Slot{
					RoomID:    roomID,
					StartTime: startOfSlot,
					EndTime:   endOfSlot,
				})

				// Сдвигаем начало следующего слота
				startOfSlot = endOfSlot
			}
		}
	}

	return slots, nil
}
