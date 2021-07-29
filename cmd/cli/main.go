package main

import (
	"fmt"
	"log"
	"mapdownloader/internal/downloader"
	"reflect"
	"strconv"
	"time"

	"code.cloudfoundry.org/bytefmt"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func main() {
	p := tea.NewProgram(new(model))
	if err := p.Start(); err != nil {
		log.Fatal(err)
	}
}

type model struct {
	step          int
	percent       float64
	tilesCount    int
	progress      *progress.Model
	textInput     textinput.Model
	helpTextStyle func(str string) string
}

func (m *model) Init() tea.Cmd {
	ti := textinput.NewModel()
	ti.Placeholder = "eyJ0eXBlIjoiMCIsIm..."
	ti.Focus()
	ti.CharLimit = 500
	ti.Width = 180

	prog, _ := progress.NewModel(progress.WithScaledGradient("#FF7CCB", "#FDFF8C"))

	m.textInput = ti
	m.progress = prog
	m.percent = 0.0
	m.helpTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
	m.step = 0

	return tickCmd()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if reflect.TypeOf(msg).String() == "tea.KeyMsg" {
		mg := msg.(tea.KeyMsg)
		if mg.Type == tea.KeyCtrlC || mg.Type == tea.KeyEsc {
			return m, tea.Quit
		} else if mg.Type == tea.KeyEnter {
			m.step = 1
			go m.startDownload()
			return m, nil
		} else {
			text, cmd := m.textInput.Update(msg)
			m.textInput = text
			return m, cmd
		}
	} else if reflect.TypeOf(msg).String() == "time.Time" {
		return m, tickCmd()
	}
	return m, nil
}

func (m *model) startDownload() {
	m.step = 2
	dl := downloader.NewDownLoader(downloader.MapInfo{
		Type:     1,
		MinZ:     13,
		MaxZ:     14,
		DbPath:   "./mapTiles.db",
		MinLng:   "115.962982",
		MaxLng:   "116.821289",
		MinLat:   "40.22034",
		MaxLat:   "39.692037",
		Language: "zh_CN",
	}, 4096, 4096, 512)
	m.tilesCount = dl.GetTaskInfo()
	time.Sleep(time.Second * 3)
	m.step = 3
	go dl.Start()
	for {
		time.Sleep(time.Millisecond * 500)
		m.percent = dl.GetDownPercent()
		if m.percent >= float64(1) {
			fmt.Println("done!")
			return
		}
	}
}

func (m *model) View() (str string) {
	str = "Enter the configuration string from the website <map.lizhengtech.com> \n\n"
	switch m.step {
	case 0:
		str += m.textInput.View() + "\n\n"
	case 1:
		str += "process..."
	case 2:
		str += " Guage Size: " + bytefmt.ByteSize(uint64(m.tilesCount*1024*15)) + "\n\n" +
			"Tiles Count: " + strconv.Itoa(m.tilesCount) + "\n\n" +
			"Start Download ..." + "\n\n"
	case 3:
		str += m.progress.View(m.percent)
	default:
		return fmt.Sprintf("err")
	}
	str += m.helpTextStyle("(esc to quit)") + "\n"
	return
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg {
		return time.Time(t)
	})
}
