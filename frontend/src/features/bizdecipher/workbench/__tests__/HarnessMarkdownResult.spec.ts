import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import HarnessMarkdownResult from '../HarnessMarkdownResult.vue'

describe('Harness markdown result', () => {
  it('renders long-form markdown and strips executable markup', () => {
    const wrapper = mount(HarnessMarkdownResult, {
      props: {
        content: '# 结果\n\n```ts\nconst value = 42\n```\n\n<script>window.__unsafe = true</script>\n\n[危险链接](javascript:alert(1))',
      },
    })

    expect(wrapper.get('h1').text()).toBe('结果')
    expect(wrapper.get('code').text()).toContain('const value = 42')
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.find('a').attributes('href')).toBeUndefined()
  })
})
