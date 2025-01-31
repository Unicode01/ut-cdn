// file render the server

var api_url = "";
var serverData = "";

function init_api(){
    //enter api url check & input
    if (!check_if_api_saved()) {
        //render the api input form
        render_api_input_form();
    }else {
        api_url = localStorage.getItem("api_url");
    }
    //end api url check & input

    //render api info
    get_api_info();
        
}

function check_if_api_saved(){
    // check if api is saved
    var api_url = localStorage.getItem("api_url");
    if(api_url == null){
        return false;
    }
    return true;
}

function render_api_input_form() {
    // 创建遮罩层
    const overlay = document.createElement('div');
    overlay.classList.add('overlay');
    overlay.style.position = 'fixed';
    overlay.style.top = '0';
    overlay.style.left = '0';
    overlay.style.width = '100vw';
    overlay.style.height = '100vh';
    overlay.style.backgroundColor = 'rgba(0, 0, 0, 0.5)'; // 半透明黑色背景
    overlay.style.display = 'flex';
    overlay.style.justifyContent = 'center';
    overlay.style.alignItems = 'center';
    overlay.style.zIndex = '9999'; // 确保在最顶层

    // 创建表单容器
    const formContainer = document.createElement('div');
    formContainer.style.backgroundColor = 'white';
    formContainer.style.padding = '20px';
    formContainer.style.borderRadius = '8px';
    formContainer.style.boxShadow = '0 2px 10px rgba(0, 0, 0, 0.1)';
    formContainer.style.width = '400px';
    formContainer.style.display = 'flex';
    formContainer.style.flexDirection = 'column';
    formContainer.style.gap = '15px';

    // 创建标题
    const title = document.createElement('h2');
    title.textContent = '输入API URL';
    title.style.margin = '0';
    title.style.color = '#333';
    title.style.fontSize = '1.5rem';

    // 创建输入框
    const input = document.createElement('input');
    input.type = 'text';
    input.placeholder = '请输入API地址...';
    input.id = 'apiUrlInput';
    input.style.padding = '10px';
    input.style.border = '1px solid #ddd';
    input.style.borderRadius = '4px';
    input.style.fontSize = '1rem';

    // 创建保存按钮
    const saveButton = document.createElement('button');
    saveButton.textContent = '保存URL';
    saveButton.style.padding = '10px 20px';
    saveButton.style.backgroundColor = '#4CAF50';
    saveButton.style.color = 'white';
    saveButton.style.border = 'none';
    saveButton.style.borderRadius = '4px';
    saveButton.style.cursor = 'pointer';
    saveButton.style.fontSize = '1rem';
    saveButton.onclick = () => {
        // 调用保存方法并传递输入值
        const url = document.getElementById('apiUrlInput').value;
        console.log('保存的URL:', url);
        localStorage.setItem('api_url', url);
        api_url = url;
        // 关闭表单
        overlay.remove();
        init_api();
    };

    // 添加悬停效果
    saveButton.addEventListener('mouseover', () => {
        saveButton.style.backgroundColor = '#45a049';
    });
    saveButton.addEventListener('mouseout', () => {
        saveButton.style.backgroundColor = '#4CAF50';
    });

    // 组装元素
    formContainer.appendChild(title);
    formContainer.appendChild(input);
    formContainer.appendChild(saveButton);
    overlay.appendChild(formContainer);

    // 添加到页面
    document.body.appendChild(overlay);
}


function get_api_info(){
     // send request to api to get server info
     const xhr = new XMLHttpRequest();
     xhr.open('GET', api_url);
     xhr.onload = function() {
         if (xhr.status === 200) {
             serverData = JSON.parse(xhr.responseText);
             render_info();
         } else {
             console.error('Error:', xhr.status);
             return "";
         }
     };
     xhr.send();
    return;
}

function render_info() {
    // 初始化仪表盘
    function initDashboard() {
        // 强化数据转换
        const dataEntries = Object.entries(serverData.DataTransferred).map(([key, value]) => {
            // 添加类型安全检查
            if (typeof value !== 'number') {
                console.warn(`非数字值: ${key}=${value}`);
                return [key, 0];
            }
            return [key, value];
        });
        const dataEntries_Requests = Object.entries(serverData.Requests).map(([key, value]) => {
            // 添加类型安全检查
            if (typeof value !== 'number') {
                console.warn(`非数字值: ${key}=${value}`);
                return [key, 0];
            }
            return [key, value];
        });

        // 正确计算总传输量
        const totalBytes = dataEntries.reduce((sum, [_, val]) => sum + val, 0);
        document.getElementById('total-transferred').textContent = formatBytes(totalBytes);
        // 总请求量计算
        const totalRequests = Object.values(serverData.Requests).reduce((a, b) => a + b, 0);
        document.getElementById('total-requests').textContent = formatNumber(totalRequests);
        // 填充核心指标
        document.getElementById('requests').textContent = formatNumber(totalRequests);
        document.getElementById('errors').textContent = serverData.Errors;
        document.getElementById('clients').textContent = serverData.ActiveClients;
        document.getElementById('cpu').innerHTML = `${formatNumber(serverData.CPU_Time / 1e9)}<span class="metric-unit">s</span>`; ;
        document.getElementById('total-requests').textContent = formatNumber(totalRequests);

        // 平均CPU时间计算
        const avgCpuElement = document.getElementById('avg-cpu');
        if (totalRequests > 0) {
            const avgNs = serverData.CPU_Time / totalRequests;
            const avgMs = (avgNs / 1e6).toFixed(3); // 转换为毫秒并保留3位小数
            avgCpuElement.innerHTML = `${avgMs}<span class="metric-unit">ms/req</span>`;
        } else {
            avgCpuElement.textContent = 'N/A';
        }

        // 填充启动时间
        document.getElementById('start-time').textContent = formatTimestamp(serverData.StartTime);

         // 强制数字类型排序
        const sortedData = dataEntries.sort((a, b) => Number(b[1]) - Number(a[1]));
        const sortedData_Requests = dataEntries_Requests.sort((a, b) => Number(b[1]) - Number(a[1]));

        // 填充数据传输表格
        populateTable(
            '#data-transferred tbody',
            sortedData,
            formatBytes
            
        );
        // 填充请求数表格
        populateTable(
            '#requests tbody',
            sortedData_Requests,
            formatNumber
    
        );
        // 填充IP统计表格
        populateTable(
            '#ip-stats tbody',
            Object.entries(serverData.IPs),
            formatNumber
        );
    }

    // 修改后的通用表格填充函数
    function populateTable(selector, data, formatter) {
        const tbody = document.querySelector(selector);
        tbody.innerHTML = data.map(([key, value]) => `
            <tr>
                <td>${key}</td>
                <td class="number">${formatter(value)}</td>
            </tr>
        `).join('');
    }

    // 数字格式化函数
    function formatNumber(num) {
        return num.toLocaleString('en-US');
    }

    // 字节格式化函数
    function formatBytes(bytes) {
        if (!Number.isFinite(bytes)) return '0 B';
        
        const units = ['B', 'KB', 'MB', 'GB', 'TB'];
        let unitIndex = 0;
        let converted = bytes;
        
        while (converted >= 1024 && unitIndex < units.length - 1) {
            converted /= 1024;
            unitIndex++;
        }
        
        // 精确四舍五入
        return `${converted.toFixed(2)} ${units[unitIndex]}`;
    }

    // 时间戳格式化函数
    function formatTimestamp(timestamp) {
        const date = new Date(timestamp * 1000);
        return date.toLocaleString('zh-CN', {
            year: 'numeric',
            month: '2-digit',
            day: '2-digit',
            hour: '2-digit',
            minute: '2-digit',
            second: '2-digit'
        });
}
initDashboard();
}


init_api();