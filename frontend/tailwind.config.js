/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js,ts,jsx,tsx}'],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Gateway Console 品牌色 —— 清爽靛蓝（替换原本沉郁的重紫 #7C5CFF，
        // 紫+死黑读起来压抑；靛蓝更通透、亲和，仍属冷色控制台调性）。
        primary: {
          DEFAULT: '#6E8BFF',
          50: '#EEF1FF',
          100: '#E0E6FF',
          200: '#C4CEFF',
          300: '#A5B4FF',
          400: '#8499FF',
          500: '#6E8BFF',
          600: '#5570F0',
          700: '#4658D6',
          800: '#3A48AD',
          900: '#313D88',
          950: '#1F2552'
        },
        // 深色控制台背景 —— 从近纯黑 #080A0F 抬升为带冷蓝调的「深板岩 Slate」，
        // 更透气、不压抑（参考 Linear / Vercel 的深色面，而非死黑）。
        console: {
          bg: '#0F1421',
          panel: '#161C2C',
          'panel-2': '#1E2638',
          'panel-3': '#28324A'
        },
        // 状态色
        success: '#22C55E',
        warning: '#F5B942',
        danger: '#FF5A5F',
        info: '#5B9BFF',
        // 文字色
        text: {
          DEFAULT: '#EEF1F8',
          muted: '#8794AE',
          disabled: '#4A5573'
        },
        // 边框
        border: {
          DEFAULT: 'rgba(255,255,255,0.08)',
          hover: 'rgba(255,255,255,0.12)',
          active: 'rgba(255,255,255,0.16)'
        },
        // dark-* 色板：原本是 one-api 残留的蓝灰 slate，全站 1600+ 处以
        // `gray-X dark:dark-Y` 配对方式使用（深色下仅 dark:dark-Y 生效）。
        // 整条色阶与 console 深板岩 Slate 对齐，并严格保持
        // light(50) → dark(950) 的档位顺序，因此所有旧用法无需逐个改动即可
        // 自动跟随新设计系统：
        //   text:        dark-100 / dark-200  (近白正文)
        //   muted:       dark-400            (= console text.muted #8794AE)
        //   border:      dark-600 / dark-700  (深板岩上的细线)
        //   surface/bg:  dark-800(panel-2) / dark-900(panel) / dark-950(bg)
        dark: {
          50: '#F3F5FA',
          100: '#E7EBF3',
          200: '#CDD5E3',
          300: '#A8B3C9',
          400: '#8794AE',
          500: '#687596',
          600: '#3D4763',
          700: '#28324A',
          800: '#1E2638',
          900: '#161C2C',
          950: '#0F1421'
        }
      },
      fontFamily: {
        sans: ['Inter', 'Noto Sans SC', '-apple-system', 'BlinkMacSystemFont', 'sans-serif'],
        mono: ['JetBrains Mono', 'Fira Code', 'ui-monospace', 'SFMono-Regular', 'monospace']
      },
      boxShadow: {
        glass: '0 8px 32px rgba(0, 0, 0, 0.08)',
        'glass-sm': '0 4px 16px rgba(0, 0, 0, 0.06)',
        glow: '0 0 20px rgba(110, 139, 255, 0.25)',
        'glow-lg': '0 0 40px rgba(110, 139, 255, 0.35)',
        card: '0 1px 3px rgba(0, 0, 0, 0.04), 0 1px 2px rgba(0, 0, 0, 0.06)',
        'card-hover': '0 10px 40px rgba(0, 0, 0, 0.08)',
        'inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1)'
      },
      backgroundImage: {
        'gradient-radial': 'radial-gradient(var(--tw-gradient-stops))',
        'gradient-primary': 'linear-gradient(135deg, #6E8BFF 0%, #4658D6 100%)',
        'gradient-dark': 'linear-gradient(135deg, #1E2638 0%, #0F1421 100%)',
        'gradient-glass':
          'linear-gradient(135deg, rgba(255,255,255,0.1) 0%, rgba(255,255,255,0.05) 100%)',
        'mesh-gradient':
          'radial-gradient(at 40% 20%, rgba(110, 139, 255, 0.10) 0px, transparent 50%), radial-gradient(at 80% 0%, rgba(110, 139, 255, 0.06) 0px, transparent 50%), radial-gradient(at 0% 50%, rgba(110, 139, 255, 0.08) 0px, transparent 50%)'
      },
      animation: {
        'fade-in': 'fadeIn 0.3s ease-out',
        'slide-up': 'slideUp 0.3s ease-out',
        'slide-down': 'slideDown 0.3s ease-out',
        'slide-in-right': 'slideInRight 0.3s ease-out',
        'scale-in': 'scaleIn 0.2s ease-out',
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
        shimmer: 'shimmer 2s linear infinite',
        glow: 'glow 2s ease-in-out infinite alternate'
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' }
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-10px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' }
        },
        slideInRight: {
          '0%': { opacity: '0', transform: 'translateX(20px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' }
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' }
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' }
        },
        glow: {
          '0%': { boxShadow: '0 0 20px rgba(110, 139, 255, 0.25)' },
          '100%': { boxShadow: '0 0 30px rgba(110, 139, 255, 0.4)' }
        }
      },
      backdropBlur: {
        xs: '2px'
      },
      borderRadius: {
        '4xl': '2rem'
      }
    }
  },
  plugins: []
}
