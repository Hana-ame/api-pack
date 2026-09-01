// URL-safe Base64 编码示例
// 用于生成 params 查询参数

/**
 * 将对象转换为 URL-safe base64 编码字符串
 * @param {Object} obj - 要编码的对象
 * @returns {string} URL-safe base64 编码的字符串（无填充）
 */
function encodeParams(obj) {
  const jsonStr = JSON.stringify(obj);
  return btoa(jsonStr)
    .replace(/\+/g, '-')  // + -> -
    .replace(/\//g, '_')  // / -> _
    .replace(/=/g, '');   // 移除填充字符 =
}

/**
 * 从 URL-safe base64 字符串解码为对象
 * @param {string} encoded - URL-safe base64 编码的字符串
 * @returns {Object} 解码后的对象
 */
function decodeParams(encoded) {
  // 恢复标准 base64
  let base64 = encoded
    .replace(/-/g, '+')  // - -> +
    .replace(/_/g, '/'); // _ -> /
  
  // 添加填充
  while (base64.length % 4 !== 0) {
    base64 += '=';
  }
  
  const jsonStr = atob(base64);
  return JSON.parse(jsonStr);
}

// ===== 使用示例 =====

// 示例 1: 基本参数
const params1 = {
  host: "example.com",
  scheme: "https"
};
const encoded1 = encodeParams(params1);
console.log("示例 1:", encoded1);
// 输出: eyJob3N0IjoiZXhhbXBsZS5jb20iLCJzY2hlbWUiOiJodHRwcyJ9

// 示例 2: 完整参数
const params2 = {
  host: "api.example.com",
  referer: "https://myapp.com",
  scheme: "https",
  origin: "https://myapp.com"
};
const encoded2 = encodeParams(params2);
console.log("示例 2:", encoded2);

// 构建完整 URL
const baseUrl = "http://localhost:8080/test/path";
const fullUrl = `${baseUrl}?params=${encoded2}&other=value`;
console.log("完整 URL:", fullUrl);

// 验证解码
const decoded = decodeParams(encoded2);
console.log("解码结果:", decoded);

// 在浏览器中使用
// fetch(fullUrl).then(response => response.text()).then(console.log);
