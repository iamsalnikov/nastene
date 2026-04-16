// Клиентское сжатие аватара: вписываем в maxSide x maxSide пикселей и пережимаем в JPEG.
// Подменяем содержимое <input type="file"> через DataTransfer, так что форма уходит уже сжатой.
(function () {
    const MAX_SIDE = 500;
    const QUALITY = 0.85;

    function init() {
        const input = document.querySelector('input[type="file"][name="avatar"]');
        if (!input) return;
        const preview = document.querySelector('[data-avatar-preview]');
        const status = document.querySelector('[data-avatar-status]');

        input.addEventListener('change', async () => {
            if (!input.files || !input.files[0]) return;
            const file = input.files[0];
            if (!/^image\/(png|jpeg)$/.test(file.type)) {
                if (status) status.textContent = 'Поддерживаются только PNG и JPEG.';
                input.value = '';
                return;
            }
            try {
                const resized = await resize(file);
                if (resized && resized.size < file.size) {
                    const dt = new DataTransfer();
                    dt.items.add(new File([resized], 'avatar.jpg', { type: 'image/jpeg' }));
                    input.files = dt.files;
                    if (status) status.textContent = `сжато: ${formatSize(file.size)} → ${formatSize(resized.size)}`;
                } else if (status) {
                    status.textContent = `файл: ${formatSize(file.size)}`;
                }
                if (preview) preview.src = URL.createObjectURL(input.files[0]);
            } catch (e) {
                if (status) status.textContent = 'Не удалось обработать картинку: ' + e.message;
            }
        });
    }

    function resize(file) {
        return new Promise((resolve, reject) => {
            const url = URL.createObjectURL(file);
            const img = new Image();
            img.onload = () => {
                URL.revokeObjectURL(url);
                const { width, height } = fit(img.width, img.height, MAX_SIDE);
                const canvas = document.createElement('canvas');
                canvas.width = width;
                canvas.height = height;
                const ctx = canvas.getContext('2d');
                ctx.fillStyle = '#fff';
                ctx.fillRect(0, 0, width, height);
                ctx.drawImage(img, 0, 0, width, height);
                canvas.toBlob(
                    (blob) => {
                        if (!blob) {
                            reject(new Error('toBlob вернул null'));
                            return;
                        }
                        resolve(blob);
                    },
                    'image/jpeg',
                    QUALITY
                );
            };
            img.onerror = () => {
                URL.revokeObjectURL(url);
                reject(new Error('image.onerror'));
            };
            img.src = url;
        });
    }

    function fit(w, h, max) {
        if (w <= max && h <= max) return { width: w, height: h };
        const k = Math.min(max / w, max / h);
        return { width: Math.round(w * k), height: Math.round(h * k) };
    }

    function formatSize(n) {
        if (n < 1024) return n + ' B';
        if (n < 1024 * 1024) return (n / 1024).toFixed(1) + ' KB';
        return (n / 1024 / 1024).toFixed(2) + ' MB';
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
