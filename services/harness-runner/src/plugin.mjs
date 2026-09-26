import { defineTool } from '@deepseek-ai/dsh-tools'
import { resolveSkills } from './catalog.mjs'
import { generateImage } from './media.mjs'
import { writeFile } from 'node:fs/promises'

export const name = 'bizdecipher-creation'
export const inject = ['tools', 'skills']

export function apply(ctx) {
  const selected = resolveSkills(JSON.parse(process.env.BIZ_SKILL_IDS || '[]'))
  for (const skill of selected) {
    ctx.effect(() => ctx.skills.register({
      name: skill.id,
      description: skill.description,
      content: skill.content,
      source: 'bundled',
      invocation: { userInvocable: true, modelInvocable: true },
    }))
  }
  if (!process.env.BIZ_IMAGE_KEY || !process.env.BIZ_IMAGE_MODEL) return
  let calls = 0
  ctx.effect(() => ctx.tools.register(defineTool({
    name: 'generate_image',
    description: 'Generate one real image using the selected user resource. This incurs gateway model usage. Call only when the user requested image creation; at most once per task.',
    parameters: {
      prompt: { type: 'string', required: true, description: 'Detailed image description' },
      size: { type: 'string', required: true, enum: ['1024x1024', '1024x1536', '1536x1024'] },
    },
    output: {
      schema: {
        type: 'object',
        additionalProperties: false,
        properties: {
          images: { type: 'array', required: true, items: { type: 'string' } },
          model: { type: 'string', required: true },
        },
      },
      render: (_args, value) => [{ type: 'text', text: JSON.stringify(value) }],
    },
    async execute(args, exec) {
      if (calls >= 1) throw new Error('This task has already attempted image generation. Start a new task to generate again.')
      calls += 1
      const result = await generateImage({
        baseURL: process.env.BIZ_GATEWAY_URL,
        apiKey: process.env.BIZ_IMAGE_KEY,
        model: process.env.BIZ_IMAGE_MODEL,
        prompt: args.prompt,
        size: args.size,
        signal: exec.signal,
      })
      await writeFile(process.env.BIZ_OUTPUT_FILE, JSON.stringify(result.images), { mode: 0o600 })
      return result
    },
  })))
}
