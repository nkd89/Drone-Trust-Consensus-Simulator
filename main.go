package main

import (
	"drone_trust_sim/config"
	"drone_trust_sim/metrics"
	"drone_trust_sim/simulator"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func runSingleSimulation(cfg *config.SimulatorConfig) *metrics.FinalMetrics {
	sim := simulator.NewSimulator(cfg)
	finalMetrics := sim.Run()
	return finalMetrics
}

func runSimulationBatch(baseCfg *config.SimulatorConfig, numRuns int) {
	log.Printf("--- Запуск серии симуляций для: %s (%d запусков) ---", baseCfg.AlgorithmName, numRuns)

	resultsPath := filepath.Join("simulation_results", baseCfg.ResultsDir)
	if err := os.MkdirAll(resultsPath, 0755); err != nil {
		log.Fatalf("Не удалось создать директорию для результатов: %v", err)
	}

	allMetrics := make([]*metrics.FinalMetrics, 0, numRuns)
	for i := 0; i < numRuns; i++ {
		// Убираем лог для чистоты вывода, если запусков много
		// log.Printf("... Запуск %d из %d ...", i+1, numRuns)
		runCfg := *baseCfg
		metrics := runSingleSimulation(&runCfg)
		allMetrics = append(allMetrics, metrics)
	}

	// <<< НОВЫЙ ВЫЗОВ: Сохраняем все "сырые" результаты >>>
	allRunsReportPath := filepath.Join(resultsPath, "final_metrics.csv")
	if err := metrics.SaveAllMetricsToCSV(allMetrics, allRunsReportPath); err != nil {
		log.Printf("Ошибка сохранения отчета по всем запускам в CSV: %v", err)
	}

	// Усредняем результаты
	avgMetrics := metrics.AverageMetrics(allMetrics)
	avgMetrics.Print()

	// Сохраняем усредненный отчет
	avgReportPath := filepath.Join(resultsPath, "average_metrics.csv")
	if err := avgMetrics.SaveToCSV(avgReportPath); err != nil {
		log.Printf("Ошибка сохранения усредненного отчета в CSV: %v", err)
	}

	log.Printf("Серия симуляций для '%s' завершена. Отчеты сохранены в %s и %s",
		baseCfg.AlgorithmName, avgReportPath, allRunsReportPath)
}

func main() {
	const numSimulationsPerConfig = 100

	configs := []*config.SimulatorConfig{
		config.GetBaseBTMSDConfig(),
		// config.GetPoRSConfig(),
		config.GetBlockchainConfig(),
		config.GetPoRSBlockchainConfig(),
		config.GetPoRSConsensusConfig(),
		config.GetUnifiedPoRSConfig(),
	}

	// Запускаем серии симуляций для каждой конфигурации
	for _, cfg := range configs {
		runSimulationBatch(cfg, numSimulationsPerConfig)
		fmt.Println(string(make([]byte, 80, 80))) // Разделитель
	}
}
