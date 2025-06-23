package main

import (
	"drone_trust_sim/config"
	"drone_trust_sim/metrics"
	"drone_trust_sim/simulator"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// runSingleSimulation (без изменений)
func runSingleSimulation(cfg *config.SimulatorConfig) *metrics.FinalMetrics {
	sim := simulator.NewSimulator(cfg)
	finalMetrics := sim.Run()
	return finalMetrics
}

// BatchResult хранит результат выполнения одной серии симуляций
type BatchResult struct {
	Config      *config.SimulatorConfig
	AvgMetrics  *metrics.FinalMetrics
	Err         error
	ReportPaths []string
}

// runSimulationBatch теперь возвращает результат через канал
func runSimulationBatch(cfg *config.SimulatorConfig, numRuns int) *BatchResult {
	// Эта функция теперь не выводит логи, а возвращает результат
	result := &BatchResult{Config: cfg}

	resultsPath := filepath.Join("simulation_results", cfg.ResultsDir)
	if err := os.MkdirAll(resultsPath, 0755); err != nil {
		result.Err = fmt.Errorf("не удалось создать директорию: %w", err)
		return result
	}

	allMetrics := make([]*metrics.FinalMetrics, 0, numRuns)
	for i := 0; i < numRuns; i++ {
		metrics := runSingleSimulation(cfg)
		allMetrics = append(allMetrics, metrics)
	}

	avgMetrics := metrics.AverageMetrics(allMetrics)
	result.AvgMetrics = avgMetrics

	// Сохраняем отчеты
	avgReportPath := filepath.Join(resultsPath, "average_metrics.csv")
	if err := avgMetrics.SaveToCSV(avgReportPath); err != nil {
		result.Err = err
	}
	result.ReportPaths = append(result.ReportPaths, avgReportPath)

	allRunsReportPath := filepath.Join(resultsPath, "all_runs_metrics.csv")
	if err := metrics.SaveAllMetricsToCSV(allMetrics, allRunsReportPath); err != nil {
		result.Err = err
	}
	result.ReportPaths = append(result.ReportPaths, allRunsReportPath)

	return result
}

func main() {
	const numRunsPerConfig = 100 // Количество запусков для усреднения

	// --- Параллельное выполнение ---
	// Ограничиваем количество одновременно работающих "тяжелых" горутин
	// числом доступных ядер процессора.
	maxConcurrentBatches := runtime.NumCPU()
	log.Printf("Используется %d потоков для параллельного выполнения серий симуляций.", maxConcurrentBatches)

	log.Println("Генерация плана эксперимента (DOE)...")
	experimentConfigs := config.GenerateExperimentConfigs()
	log.Printf("План сгенерирован. Всего конфигураций для теста: %d", len(experimentConfigs))

	startTime := time.Now()

	var wg sync.WaitGroup
	// Семафор для ограничения параллелизма
	semaphore := make(chan struct{}, maxConcurrentBatches)
	// Канал для сбора результатов
	resultsChan := make(chan *BatchResult, len(experimentConfigs))

	// Запускаем горутины для каждой конфигурации
	for _, cfg := range experimentConfigs {
		wg.Add(1)
		// Захватываем место в семафоре
		semaphore <- struct{}{}

		go func(config *config.SimulatorConfig) {
			defer wg.Done()
			// Освобождаем место в семафоре после завершения
			defer func() { <-semaphore }()

			// Выполняем серию симуляций и отправляем результат в канал
			resultsChan <- runSimulationBatch(config, numRunsPerConfig)
		}(cfg)
	}

	// Ждем завершения всех горутин
	wg.Wait()
	close(resultsChan) // Закрываем канал после того, как все горутины отработали

	// Обрабатываем результаты из канала
	log.Println("\n===== ОБРАБОТКА РЕЗУЛЬТАТОВ =====")
	successCount := 0
	errorCount := 0
	for result := range resultsChan {
		if result.Err != nil {
			errorCount++
			log.Printf("ОШИБКА в серии '%s': %v", result.Config.AlgorithmName, result.Err)
		} else {
			successCount++
			log.Printf("УСПЕХ: Серия для '%s' (Drones: %d, Malicious: %.1f) завершена. Отчеты сохранены.",
				result.Config.AlgorithmName, result.Config.NumDrones, result.Config.MaliciousRatio)
		}
	}

	duration := time.Since(startTime)
	log.Printf("\n\n===== ВЕСЬ ЭКСПЕРИМЕНТ ЗАВЕРШЕН! =====")
	log.Printf("Общее время выполнения: %s", duration)
	log.Printf("Успешно выполненных серий: %d", successCount)
	log.Printf("Серий с ошибками: %d", errorCount)
}
