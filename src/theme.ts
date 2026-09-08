import type { ThemeConfig } from "antd";
import { theme } from "antd";

export const appTheme: ThemeConfig = {
  algorithm: theme.darkAlgorithm,
  token: {
    colorPrimary: "#c4a35a",
    colorInfo: "#c4a35a",
    colorSuccess: "#7ea36b",
    colorError: "#c45c48",
    colorWarning: "#c4a35a",
    colorBgBase: "#12110e",
    colorBgContainer: "#1b1914",
    colorBgElevated: "#221f18",
    colorBgLayout: "#12110e",
    colorBorder: "#3a3429",
    colorBorderSecondary: "#2a261e",
    colorText: "#efe6d4",
    colorTextSecondary: "#9a8f7a",
    colorTextTertiary: "#7a705e",
    borderRadius: 8,
    fontFamily: '"IBM Plex Sans", "Segoe UI", sans-serif',
    fontFamilyCode: '"IBM Plex Mono", ui-monospace, monospace',
  },
  components: {
    Layout: {
      siderBg: "#16140f",
      headerBg: "#16140f",
      bodyBg: "#12110e",
    },
    Menu: {
      darkItemBg: "#16140f",
      darkItemSelectedBg: "#2a2418",
      darkItemSelectedColor: "#e2c98a",
      darkItemColor: "#9a8f7a",
      darkItemHoverBg: "#221e16",
    },
    Button: {
      primaryShadow: "none",
    },
    Table: {
      headerBg: "#1b1914",
      colorBgContainer: "#1b1914",
    },
    Drawer: {
      colorBgElevated: "#1b1914",
    },
    Modal: {
      contentBg: "#1b1914",
      headerBg: "#1b1914",
    },
    Input: {
      colorBgContainer: "#16140f",
    },
    Select: {
      colorBgContainer: "#16140f",
    },
  },
};

export const noAuto = {
  autoComplete: "off" as const,
  autoCorrect: "off",
  autoCapitalize: "off",
  spellCheck: false,
  "data-lpignore": "true",
  "data-1p-ignore": "true",
  "data-bwignore": "true",
  "data-form-type": "other",
};
