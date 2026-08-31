document.addEventListener('click', (e) => {
    if (e.target.closest('a')) return;

    const row = e.target.closest('tr[data-href]');
    if (row) window.location = row.dataset.href;
});
