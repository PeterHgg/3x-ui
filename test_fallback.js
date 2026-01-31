const axios = require('axios');

async function testFallback(url) {
  try {
    console.log(`1. 请求短链接: ${url}`);
    const response = await axios.get(url, {
      headers: {
        'User-Agent': 'Mozilla/5.0 (iPhone; CPU iPhone OS 16_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Mobile/15E148 Safari/604.1',
      },
      maxRedirects: 0, // 禁止自动重定向，手动处理
      validateStatus: status => status >= 200 && status < 400
    });

    // 这里的逻辑通常不会触发，因为抖音短链接会返回 302
    console.log('响应状态:', response.status);
  } catch (error) {
    if (error.response && (error.response.status === 301 || error.response.status === 302)) {
      const longUrl = error.response.headers.location;
      console.log(`2. 获取到重定向长链接: ${longUrl}`);

      // 请求长链接获取 HTML
      await fetchHtmlAndExtract(longUrl);
    } else {
      console.error('请求失败:', error.message);
    }
  }
}

async function fetchHtmlAndExtract(longUrl) {
  try {
    console.log('3. 请求长链接获取 HTML...');
    const response = await axios.get(longUrl, {
      headers: {
        'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        'Cookie': 's_v_web_id=verify_layvk80b_o1234567_1234_1234_1234_123456789012;' // 随便模拟一个 cookie 试试，有时不需要
      }
    });

    const html = response.data;
    console.log(`4. 获取到 HTML (长度: ${html.length})`);

    // 尝试正则匹配 json 数据
    // 抖音网页通常把数据放在 id="RENDER_DATA" 的 script 标签里，或者 _SSR_DATA
    const renderDataMatch = html.match(/<script id="RENDER_DATA" type="application\/json">([\s\S]*?)<\/script>/);

    if (renderDataMatch) {
        console.log('5. 找到 RENDER_DATA');
        try {
            const dataStr = decodeURIComponent(renderDataMatch[1]);
            const data = JSON.parse(dataStr);
            // 尝试寻找视频地址
            // 路径通常很深，这里只是做一个简单的查找示例
            // 实际路径可能在 data.app.videoDetail...
            console.log('数据解析成功，正在查找视频链接...');
            // 简单粗暴查找 http 链接
            const jsonStr = JSON.stringify(data);
            const videoMatch = jsonStr.match(/"playApi":"(https:\\\/\\\/[^"]+)"/);
            if (videoMatch) {
                console.log('SUCCESS! 找到视频地址:', videoMatch[1].replace(/\\/g, ''));
            } else {
                console.log('未在 JSON 中直接匹配到 playApi，尝试其他字段...');
                // 可能是 video.play_addr.url_list
            }
        } catch (e) {
            console.error('JSON 解析失败', e);
        }
    } else {
        console.log('未找到 RENDER_DATA，尝试直接正则匹配 src...');
        // 尝试匹配 <video> 标签或 src
        const srcMatch = html.match(/src="(https:[^"]+v26[^"]+)"/); // 抖音视频链接通常包含 v26
        if (srcMatch) {
             console.log('SUCCESS! 正则匹配到可能的视频地址:', srcMatch[1]);
        } else {
            console.log('正则匹配失败');
        }
    }

  } catch (error) {
    console.error('获取 HTML 失败:', error.message);
  }
}

// 使用一个抖音链接测试
testFallback('https://v.douyin.com/cBvneIqHohc/');
