/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-09 22:42:49
 * @FilePath: \electron-go-app\frontend\tailwind.config.ts
 * @LastEditTime: 2025-10-09 22:42:53
 */
import type { Config } from "tailwindcss";
import plugin from "tailwindcss/plugin";

const config: Config = {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx,js,jsx}"],
  theme: {
    extend: {
      colors: {
        background: "var(--bg)",
        foreground: "var(--fg)",
        primary: {
          DEFAULT: "#3B5BDB",
          foreground: "#FFFFFF"
        },
        secondary: {
          DEFAULT: "#5F3DC4",
          foreground: "#FFFFFF"
        },
        muted: {
          DEFAULT: "#EDF2FF",
          foreground: "#344054"
        },
        success: "#12B886",
        warning: "#F08C00",
        destructive: "#E03131"
      },
      boxShadow: {
        glow: "0 0 0 4px rgba(59, 91, 219, 0.15)",
        elevation: "0 18px 40px rgba(56, 56, 122, 0.12)"
      },
      backdropBlur: {
        xl: "20px"
      },
      keyframes: {
        shimmer: {
          "0%": { backgroundPosition: "-468px 0" },
          "100%": { backgroundPosition: "468px 0" }
        },
        "toast-in": {
          "0%": { opacity: "0", transform: "translateY(12px)" },
          "100%": { opacity: "1", transform: "translateY(0)" }
        }
      },
      animation: {
        shimmer: "shimmer 1.2s infinite linear",
        "toast-in": "toast-in 180ms ease-out forwards"
      }
    }
  },
  plugins: [
    plugin(({ addComponents }) => {
      addComponents({
        ".glass": {
          backgroundColor: "rgba(255, 255, 255, 0.7)",
          backdropFilter: "blur(20px)",
          border: "1px solid rgba(255, 255, 255, 0.4)"
        }
      });
    })
  ]
};

export default config;
