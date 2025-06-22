package metrics

import (
	"drone_trust_sim/config"
	"drone_trust_sim/models"
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

// TrustManagerReader описывает, что нужно от менеджера доверия
type TrustManagerReader interface {
	GetTrust(observerID, targetID int) float64
}

// SimulationResultProvider описывает, что нужно от симулятора для финального отчета
type SimulationResultProvider interface {
	GetNodes() []*models.DroneNode
	GetTrustManagerForMetrics() TrustManagerReader
	GetConfig() *config.SimulatorConfig
	GetSimulationTime() float64
}

type FinalMetrics struct {
	AlgorithmName    string
	PDR              float64 // Packet Delivery Ratio
	MeanDelay        float64
	EnergyEfficiency float64 // Delivered packets per Joule
	CHChurnRate      float64 // Смены CH в минуту
	FalsePositives   int
	FalseNegatives   int
}

func (mc *Collector) CalculateFinalMetrics(simResultProvider SimulationResultProvider) *FinalMetrics {
	mc.Lock()
	defer mc.Unlock()

	nodes := simResultProvider.GetNodes()
	tm := simResultProvider.GetTrustManagerForMetrics()
	cfg := simResultProvider.GetConfig()
	simulationTime := simResultProvider.GetSimulationTime()

	fm := &FinalMetrics{AlgorithmName: cfg.AlgorithmName}

	if mc.PacketsSent > 0 {
		fm.PDR = float64(mc.PacketsDelivered) / float64(mc.PacketsSent)
	}
	if mc.PacketsDelivered > 0 {
		fm.MeanDelay = mc.TotalDelay / float64(mc.PacketsDelivered)
	}

	var totalEnergyConsumed float64
	for _, n := range nodes {
		totalEnergyConsumed += (cfg.InitialEnergy - n.Energy)
	}

	if totalEnergyConsumed > 0 {
		fm.EnergyEfficiency = float64(mc.PacketsDelivered) / totalEnergyConsumed
	}

	simulationMinutes := simulationTime / 60.0
	if simulationMinutes > 0 {
		fm.CHChurnRate = float64(mc.CHChanges) / simulationMinutes
	}

	fp := 0
	fn := 0
	for i := range nodes {
		for j := range nodes {
			if i == j {
				continue
			}

			trustValue := tm.GetTrust(i, j)
			isConsideredTrusted := trustValue >= cfg.TrustThreshold
			isActuallyMalicious := nodes[j].IsMalicious

			if !isActuallyMalicious && !isConsideredTrusted {
				fp++
			}
			if isActuallyMalicious && isConsideredTrusted {
				fn++
			}
		}
	}
	fm.FalsePositives = fp
	fm.FalseNegatives = fn

	return fm
}

func (fm *FinalMetrics) Print() {
	fmt.Println("--- Итоговые метрики симуляции ---")
	fmt.Printf("Алгоритм: %s\n", fm.AlgorithmName)
	fmt.Printf("PDR (Packet Delivery Ratio): %.3f\n", fm.PDR)
	fmt.Printf("Средняя задержка: %.4f с\n", fm.MeanDelay)
	fmt.Printf("Энергоэффективность: %.2f пак/Дж\n", fm.EnergyEfficiency)
	fmt.Printf("Частота смены CH: %.2f смен/мин\n", fm.CHChurnRate)
	fmt.Printf("Ошибки классификации (False Positives): %d\n", fm.FalsePositives)
	fmt.Printf("Ошибки классификации (False Negatives): %d\n", fm.FalseNegatives)
	fmt.Println("---------------------------------")
}

func (fm *FinalMetrics) SaveToCSV(filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := []string{"Algorithm", "PDR", "MeanDelay", "EnergyEfficiency", "CHChurnRate", "FalsePositives", "FalseNegatives"}
	data := []string{
		fm.AlgorithmName,
		fmt.Sprintf("%.5f", fm.PDR),
		fmt.Sprintf("%.5f", fm.MeanDelay),
		fmt.Sprintf("%.3f", fm.EnergyEfficiency),
		fmt.Sprintf("%.3f", fm.CHChurnRate),
		fmt.Sprintf("%d", fm.FalsePositives),
		fmt.Sprintf("%d", fm.FalseNegatives),
	}

	if err := writer.Write(header); err != nil {
		return err
	}
	if err := writer.Write(data); err != nil {
		return err
	}

	return nil
}

// <<< НОВАЯ ФУНКЦИЯ >>>
// SaveAllMetricsToCSV сохраняет ВСЕ метрики из серии запусков в один CSV файл.
func SaveAllMetricsToCSV(allMetrics []*FinalMetrics, filePath string) error {
	if len(allMetrics) == 0 {
		return fmt.Errorf("нет данных для сохранения")
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("не удалось создать файл: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Записываем заголовок
	header := []string{"Run", "Algorithm", "PDR", "MeanDelay", "EnergyEfficiency", "CHChurnRate", "FalsePositives", "FalseNegatives"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Записываем данные для каждого запуска
	for i, fm := range allMetrics {
		record := []string{
			fmt.Sprintf("%d", i+1), // Номер запуска (Run)
			fm.AlgorithmName,
			fmt.Sprintf("%.5f", fm.PDR),
			fmt.Sprintf("%.5f", fm.MeanDelay),
			fmt.Sprintf("%.3f", fm.EnergyEfficiency),
			fmt.Sprintf("%.3f", fm.CHChurnRate),
			fmt.Sprintf("%d", fm.FalsePositives),
			fmt.Sprintf("%d", fm.FalseNegatives),
		}
		if err := writer.Write(record); err != nil {
			// Можно просто залогировать и продолжить, чтобы не терять весь файл из-за одной строки
			log.Printf("Ошибка записи строки %d в CSV: %v", i+1, err)
		}
	}
	return nil
}

func AverageMetrics(allMetrics []*FinalMetrics) *FinalMetrics {
	if len(allMetrics) == 0 {
		return &FinalMetrics{}
	}

	// Итоговая структура для усредненных значений
	avg := &FinalMetrics{
		AlgorithmName: allMetrics[0].AlgorithmName, // Название берем из первого запуска
	}

	numMetrics := float64(len(allMetrics))
	var sumPDR, sumDelay, sumEnergy, sumChurn float64
	var sumFP, sumFN int

	for _, m := range allMetrics {
		sumPDR += m.PDR
		sumDelay += m.MeanDelay
		sumEnergy += m.EnergyEfficiency
		sumChurn += m.CHChurnRate
		sumFP += m.FalsePositives
		sumFN += m.FalseNegatives
	}

	avg.PDR = sumPDR / numMetrics
	avg.MeanDelay = sumDelay / numMetrics
	avg.EnergyEfficiency = sumEnergy / numMetrics
	avg.CHChurnRate = sumChurn / numMetrics
	// Для целочисленных значений тоже считаем среднее, но приводим к int
	avg.FalsePositives = int(float64(sumFP) / numMetrics)
	avg.FalseNegatives = int(float64(sumFN) / numMetrics)

	return avg
}
