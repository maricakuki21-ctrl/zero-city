import { describe, expect, it } from 'vitest'
import { storyChoiceResponse, storyEnding } from './starterStoryOutcomes'

describe('starter story rules', () => {
  it('distinguishes evidence-based and mistaken deductions', () => {
    expect(storyEnding('last-letter', ['', '', '档案员重新投递了旧信']).title).toBe('结局 · 迟到的告别')
    expect(storyEnding('last-letter', ['', '', '邮差带来了未来的信']).title).toBe('结局 · 尚未寄达的真相')
  })
  it('makes preparation affect the cooperative rescue ending', () => {
    const final = '分工复位道岔再一起出发'
    expect(storyEnding('fog-station', ['', '检修电源和线路', final]).title).toBe('结局 · 一起回城')
    expect(storyEnding('fog-station', ['', '记录敲击声寻找规律', final]).title).toBe('结局 · 第二班救援')
    expect(storyEnding('fog-station', ['', '', '把灯留给求助者']).title).toBe('结局 · 留在雾里的灯')
    expect(storyEnding('fog-station', ['', '', '先返回城市召集救援']).title).toBe('结局 · 带着坐标归来')
  })
  it('responds distinctly to every choice in each scene', () => {
    for (const story of ['last-letter', 'fog-station']) {
      for (let scene = 0; scene < 3; scene++) {
        const responses = [0, 1, 2].map(choice => storyChoiceResponse(story, scene, choice))
        expect(responses.every(Boolean)).toBe(true)
        expect(new Set(responses).size).toBe(3)
      }
    }
  })
})
