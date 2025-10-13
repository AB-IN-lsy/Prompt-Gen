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
