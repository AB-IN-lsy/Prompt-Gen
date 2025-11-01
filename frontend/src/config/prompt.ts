/**
 * Prompt 相关的前端配置项。
 *
 * 通过 `VITE_PROMPT_KEYWORD_LIMIT` 环境变量控制关键词数量上限，
 * 若未配置或填写非法值，则回落到与后端一致的默认 10。
 */
const rawLimit = Number.parseInt(
  import.meta.env.VITE_PROMPT_KEYWORD_LIMIT ?? "",
  10,
);

export const PROMPT_KEYWORD_LIMIT =
  Number.isInteger(rawLimit) && rawLimit > 0 ? rawLimit : 10;

const rawKeywordMaxLength = Number.parseInt(
  import.meta.env.VITE_PROMPT_KEYWORD_MAX_LENGTH ?? "",
  10,
);

export const PROMPT_KEYWORD_MAX_LENGTH =
  Number.isInteger(rawKeywordMaxLength) && rawKeywordMaxLength > 0
    ? rawKeywordMaxLength
    : 32;

const rawTagLimit = Number.parseInt(
  import.meta.env.VITE_PROMPT_TAG_LIMIT ?? "",
  10,
);

export const PROMPT_TAG_LIMIT =
  Number.isInteger(rawTagLimit) && rawTagLimit > 0 ? rawTagLimit : 5;

const rawTagMaxLength = Number.parseInt(
  import.meta.env.VITE_PROMPT_TAG_MAX_LENGTH ?? "",
  10,
);

export const PROMPT_TAG_MAX_LENGTH =
  Number.isInteger(rawTagMaxLength) && rawTagMaxLength > 0
    ? rawTagMaxLength
    : 5;

const rawKeywordRowLimit = Number.parseInt(
  import.meta.env.VITE_KEYWORD_ROW_LIMIT ?? "",
  10,
);

export const KEYWORD_ROW_LIMIT =
  Number.isInteger(rawKeywordRowLimit) && rawKeywordRowLimit > 0
    ? rawKeywordRowLimit
    : 3;

const rawDefaultKeywordWeight = Number.parseInt(
  import.meta.env.VITE_DEFAULT_KEYWORD_WEIGHT ?? "",
  10,
);

export const DEFAULT_KEYWORD_WEIGHT =
  Number.isInteger(rawDefaultKeywordWeight) && rawDefaultKeywordWeight > 0
    ? rawDefaultKeywordWeight
    : 5;

const rawAIGenerateMinDuration = Number.parseInt(
  import.meta.env.VITE_AI_GENERATE_MIN_DURATION_MS ?? "",
  10,
);

export const PROMPT_AI_GENERATE_MIN_DURATION_MS =
  Number.isInteger(rawAIGenerateMinDuration) && rawAIGenerateMinDuration > 0
    ? rawAIGenerateMinDuration
    : 2000;

const rawAutosaveDelay = Number.parseInt(
  import.meta.env.VITE_PROMPT_AUTOSAVE_DELAY_MS ?? "",
  10,
);

export const PROMPT_AUTOSAVE_DELAY_MS =
  Number.isInteger(rawAutosaveDelay) && rawAutosaveDelay > 0
    ? rawAutosaveDelay
    : 10000;

const rawMyPromptPageSize = Number.parseInt(
  import.meta.env.VITE_MY_PROMPTS_PAGE_SIZE ?? "",
  10,
);

export const MY_PROMPTS_PAGE_SIZE =
  Number.isInteger(rawMyPromptPageSize) && rawMyPromptPageSize > 0
    ? rawMyPromptPageSize
    : 10;

const rawPublicPromptPageSize = Number.parseInt(
  import.meta.env.VITE_PUBLIC_PROMPT_LIST_PAGE_SIZE ?? "",
  10,
);

export const PUBLIC_PROMPT_LIST_PAGE_SIZE =
  Number.isInteger(rawPublicPromptPageSize) && rawPublicPromptPageSize > 0
    ? rawPublicPromptPageSize
    : 9;

const rawPromptCommentPageSize = Number.parseInt(
  import.meta.env.VITE_PROMPT_COMMENT_PAGE_SIZE ?? "",
  10,
);

export const PROMPT_COMMENT_PAGE_SIZE =
  Number.isInteger(rawPromptCommentPageSize) && rawPromptCommentPageSize > 0
    ? rawPromptCommentPageSize
    : 10;

const rawGenerateTemperatureDefault = Number.parseFloat(
  import.meta.env.VITE_PROMPT_GENERATE_TEMPERATURE_DEFAULT ?? "",
);

export const PROMPT_GENERATE_TEMPERATURE_DEFAULT = Number.isFinite(
  rawGenerateTemperatureDefault,
)
  ? rawGenerateTemperatureDefault
  : 0.7;

const rawGenerateTemperatureMin = Number.parseFloat(
  import.meta.env.VITE_PROMPT_GENERATE_TEMPERATURE_MIN ?? "",
);

export const PROMPT_GENERATE_TEMPERATURE_MIN = Number.isFinite(
  rawGenerateTemperatureMin,
)
  ? rawGenerateTemperatureMin
  : 0;

const rawGenerateTemperatureMax = Number.parseFloat(
  import.meta.env.VITE_PROMPT_GENERATE_TEMPERATURE_MAX ?? "",
);

export const PROMPT_GENERATE_TEMPERATURE_MAX = Number.isFinite(
  rawGenerateTemperatureMax,
)
  ? rawGenerateTemperatureMax
  : 2;

const rawGenerateTopPDefault = Number.parseFloat(
  import.meta.env.VITE_PROMPT_GENERATE_TOP_P_DEFAULT ?? "",
);

export const PROMPT_GENERATE_TOP_P_DEFAULT = Number.isFinite(
  rawGenerateTopPDefault,
)
  ? rawGenerateTopPDefault
  : 0.9;

const rawGenerateTopPMin = Number.parseFloat(
  import.meta.env.VITE_PROMPT_GENERATE_TOP_P_MIN ?? "",
);

export const PROMPT_GENERATE_TOP_P_MIN = Number.isFinite(rawGenerateTopPMin)
  ? rawGenerateTopPMin
  : 0;

const rawGenerateTopPMax = Number.parseFloat(
  import.meta.env.VITE_PROMPT_GENERATE_TOP_P_MAX ?? "",
);

export const PROMPT_GENERATE_TOP_P_MAX = Number.isFinite(rawGenerateTopPMax)
  ? rawGenerateTopPMax
  : 1;

const rawGenerateMaxOutputDefault = Number.parseInt(
  import.meta.env.VITE_PROMPT_GENERATE_MAX_OUTPUT_DEFAULT ?? "",
  10,
);

export const PROMPT_GENERATE_MAX_OUTPUT_DEFAULT =
  Number.isInteger(rawGenerateMaxOutputDefault) && rawGenerateMaxOutputDefault > 0
    ? rawGenerateMaxOutputDefault
    : 1024;

const rawGenerateMaxOutputMin = Number.parseInt(
  import.meta.env.VITE_PROMPT_GENERATE_MAX_OUTPUT_MIN ?? "",
  10,
);

export const PROMPT_GENERATE_MAX_OUTPUT_MIN =
  Number.isInteger(rawGenerateMaxOutputMin) && rawGenerateMaxOutputMin > 0
    ? rawGenerateMaxOutputMin
    : 32;

const rawGenerateMaxOutputMax = Number.parseInt(
  import.meta.env.VITE_PROMPT_GENERATE_MAX_OUTPUT_MAX ?? "",
  10,
);

export const PROMPT_GENERATE_MAX_OUTPUT_MAX =
  Number.isInteger(rawGenerateMaxOutputMax) && rawGenerateMaxOutputMax > 0
    ? rawGenerateMaxOutputMax
    : 4096;

const rawGenerateStepwiseDefault = (
  import.meta.env.VITE_PROMPT_GENERATE_STEPWISE_DEFAULT ?? ""
)
  .trim()
  .toLowerCase();

export const PROMPT_GENERATE_STEPWISE_DEFAULT =
  rawGenerateStepwiseDefault === "1" ||
  rawGenerateStepwiseDefault === "true" ||
  rawGenerateStepwiseDefault === "yes";
