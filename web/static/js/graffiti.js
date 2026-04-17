(function () {
    const canvas = document.getElementById('graffiti-canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');
    const colorInput = document.getElementById('graffiti-color');
    const sizeInput = document.getElementById('graffiti-size');
    const clearBtn = document.getElementById('graffiti-clear');
    const form = document.getElementById('graffiti-form');
    const imageField = document.getElementById('graffiti-image');

    ctx.lineJoin = 'round';
    ctx.lineCap = 'round';
    fill();

    let drawing = false;
    let last = null;

    canvas.addEventListener('mousedown', start);
    canvas.addEventListener('mousemove', move);
    window.addEventListener('mouseup', end);

    canvas.addEventListener('touchstart', e => { e.preventDefault(); start(touchPt(e)); });
    canvas.addEventListener('touchmove', e => { e.preventDefault(); move(touchPt(e)); });
    window.addEventListener('touchend', end);

    clearBtn.addEventListener('click', fill);

    form.addEventListener('submit', () => {
        imageField.value = canvas.toDataURL('image/png');
    });

    function touchPt(e) {
        const t = e.touches[0] || e.changedTouches[0];
        return { clientX: t.clientX, clientY: t.clientY };
    }

    function pt(e) {
        const rect = canvas.getBoundingClientRect();
        const sx = canvas.width / rect.width;
        const sy = canvas.height / rect.height;
        return { x: (e.clientX - rect.left) * sx, y: (e.clientY - rect.top) * sy };
    }

    function start(e) {
        drawing = true;
        last = pt(e);
    }

    function move(e) {
        if (!drawing) return;
        const p = pt(e);
        ctx.strokeStyle = colorInput.value;
        ctx.lineWidth = parseInt(sizeInput.value, 10);
        ctx.beginPath();
        ctx.moveTo(last.x, last.y);
        ctx.lineTo(p.x, p.y);
        ctx.stroke();
        last = p;
    }

    function end() {
        drawing = false;
        last = null;
    }

    function fill() {
        ctx.fillStyle = '#ffffff';
        ctx.fillRect(0, 0, canvas.width, canvas.height);
    }
})();
