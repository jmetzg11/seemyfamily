const viewer = document.querySelector('.viewer');
const full = viewer.querySelector('img');

document.addEventListener('click', (e) => {
    const img = e.target.closest('.gallery img');
    if (!img) return;

    full.src = img.src;
    full.alt = img.alt;
    full.className = img.className;
    viewer.showModal();
});

viewer.addEventListener('click', () => viewer.close());
